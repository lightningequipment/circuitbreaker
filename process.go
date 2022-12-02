package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"golang.org/x/sync/errgroup"
)

var (
	rpcTimeout = 10 * time.Second
	ctxb       = context.Background()
)

type lndclient interface {
	getIdentity() (route.Vertex, error)

	listChannels() (map[uint64]*channel, error)

	getNodeAlias(key route.Vertex) (string, error)

	subscribeHtlcEvents(ctx context.Context,
		in *routerrpc.SubscribeHtlcEventsRequest) (
		routerrpc.Router_SubscribeHtlcEventsClient, error)

	htlcInterceptor(ctx context.Context) (
		routerrpc.Router_HtlcInterceptorClient, error)

	getPendingIncomingHtlcs(ctx context.Context) (
		map[circuitKey]struct{}, error)
}

type circuitKey struct {
	channel uint64
	htlc    uint64
}

type interceptEvent struct {
	circuitKey
	valueMsat int64
	resume    func(bool) error
}

type process struct {
	client lndclient
	cfg    *config

	interceptChan chan interceptEvent
	resolveChan   chan circuitKey

	identity route.Vertex
	chanMap  map[uint64]*channel
	aliasMap map[route.Vertex]string

	peerCtrls map[route.Vertex]*peerController

	// Testing hook
	resolvedCallback func()
}

func newProcess(client lndclient, cfg *config) *process {
	return &process{
		interceptChan: make(chan interceptEvent),
		resolveChan:   make(chan circuitKey),
		chanMap:       make(map[uint64]*channel),
		aliasMap:      make(map[route.Vertex]string),
		client:        client,
		cfg:           cfg,
		peerCtrls:     make(map[route.Vertex]*peerController),
	}
}

func (p *process) run(ctx context.Context) error {
	log.Info("CircuitBreaker started")

	var err error
	p.identity, err = p.client.getIdentity()
	if err != nil {
		return err
	}

	log.Infow("Connected to lnd node",
		"pubkey", p.identity.String())

	group, ctx := errgroup.WithContext(ctx)

	stream, err := p.client.subscribeHtlcEvents(
		ctx, &routerrpc.SubscribeHtlcEventsRequest{},
	)
	if err != nil {
		return err
	}

	interceptor, err := p.client.htlcInterceptor(ctx)
	if err != nil {
		return err
	}

	log.Info("Interceptor/notification handlers registered")

	group.Go(func() error {
		err := p.processHtlcEvents(ctx, stream)
		if err != nil {
			return fmt.Errorf("htlc events error: %w", err)
		}

		return nil
	})

	group.Go(func() error {
		err := p.processInterceptor(ctx, interceptor)
		if err != nil {
			return fmt.Errorf("interceptor error: %w", err)
		}

		return err
	})

	group.Go(func() error {
		return p.eventLoop(ctx)
	})

	return group.Wait()
}

func (p *process) getPeerController(peer route.Vertex) *peerController {
	ctrl, ok := p.peerCtrls[peer]
	if ok {
		return ctrl
	}

	// If the peer does not yet exist, initialize it with no pending htlcs.
	htlcs := make(map[circuitKey]struct{})

	return p.createPeerController(peer, htlcs)
}

func (p *process) createPeerController(peer route.Vertex,
	htlcs map[circuitKey]struct{}) *peerController {

	peerCfg := p.cfg.forPeer(peer)

	alias := p.getNodeAlias(peer)

	logger := log.With(
		"peer_alias", alias,
		"peer", peer.String(),
	)

	ctrl := newPeerController(logger, peerCfg, htlcs)
	p.peerCtrls[peer] = ctrl

	return ctrl
}

func (p *process) eventLoop(ctx context.Context) error {
	// Retrieve all pending htlcs from lnd.
	allHtlcs, err := p.client.getPendingIncomingHtlcs(ctx)
	if err != nil {
		return err
	}

	// Arrange htlcs per peer.
	htlcsPerPeer := make(map[route.Vertex]map[circuitKey]struct{})
	for h := range allHtlcs {
		peer, err := p.getChanInfo(h.channel)
		if err != nil {
			return err
		}

		htlcs := htlcsPerPeer[peer.peer]
		if htlcs == nil {
			htlcs = make(map[circuitKey]struct{})
			htlcsPerPeer[peer.peer] = htlcs
		}

		htlcs[h] = struct{}{}
	}

	// Initialize peer controllers with currently pending htlcs.
	for peer, htlcs := range htlcsPerPeer {
		p.createPeerController(peer, htlcs)
	}

	for {
		select {
		case interceptEvent := <-p.interceptChan:
			chanInfo, err := p.getChanInfo(interceptEvent.channel)
			if err != nil {
				return err
			}

			ctrl := p.getPeerController(chanInfo.peer)

			if err := ctrl.process(interceptEvent); err != nil {
				return err
			}

		case resolvedKey := <-p.resolveChan:
			chanInfo, err := p.getChanInfo(resolvedKey.channel)
			if err != nil {
				return err
			}

			ctrl := p.getPeerController(chanInfo.peer)

			ctrl.resolved(resolvedKey)

			if p.resolvedCallback != nil {
				p.resolvedCallback()
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *process) processHtlcEvents(ctx context.Context,
	stream routerrpc.Router_SubscribeHtlcEventsClient) error {

	for {
		event, err := stream.Recv()
		if err != nil {
			return err
		}

		if event.EventType != routerrpc.HtlcEvent_FORWARD {
			continue
		}

		switch event.Event.(type) {
		case *routerrpc.HtlcEvent_SettleEvent:
		case *routerrpc.HtlcEvent_ForwardFailEvent:
		case *routerrpc.HtlcEvent_LinkFailEvent:

		default:
			continue
		}

		select {
		case p.resolveChan <- circuitKey{
			channel: event.IncomingChannelId,
			htlc:    event.IncomingHtlcId,
		}:

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *process) processInterceptor(ctx context.Context,
	interceptor routerrpc.Router_HtlcInterceptorClient) error {

	for {
		event, err := interceptor.Recv()
		if err != nil {
			return err
		}

		key := circuitKey{
			channel: event.IncomingCircuitKey.ChanId,
			htlc:    event.IncomingCircuitKey.HtlcId,
		}

		resume := func(resume bool) error {
			response := &routerrpc.ForwardHtlcInterceptResponse{
				IncomingCircuitKey: &routerrpc.CircuitKey{
					ChanId: key.channel,
					HtlcId: key.htlc,
				},
			}
			if resume {
				response.Action = routerrpc.ResolveHoldForwardAction_RESUME
			} else {
				response.Action = routerrpc.ResolveHoldForwardAction_FAIL
			}

			return interceptor.Send(response)
		}

		select {
		case p.interceptChan <- interceptEvent{
			circuitKey: key,
			valueMsat:  int64(event.OutgoingAmountMsat),
			resume:     resume,
		}:

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *process) getNodeAlias(key route.Vertex) string {
	alias, ok := p.aliasMap[key]
	if ok {
		return alias
	}

	alias, err := p.client.getNodeAlias(key)
	if err != nil {
		log.Warnw("cannot get node alias",
			"err", err)

		return ""
	}

	p.aliasMap[key] = alias

	return alias
}

func (p *process) getChanInfo(channel uint64) (*channel, error) {
	// Try to look up from the cache.
	ch, ok := p.chanMap[channel]
	if ok {
		return ch, nil
	}

	// Cache miss. Retrieve all channels and update the cache.
	channels, err := p.client.listChannels()
	if err != nil {
		return nil, err
	}

	for chanId, ch := range channels {
		p.chanMap[chanId] = ch
	}

	// Try looking up the channel again.
	ch, ok = p.chanMap[channel]
	if ok {
		return ch, nil
	}

	// Channel not found.
	return nil, fmt.Errorf("incoming channel %v not found", channel)
}
