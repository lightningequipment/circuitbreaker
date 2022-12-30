package main

import (
	"context"
	"fmt"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

var (
	rpcTimeout = 10 * time.Second
	ctxb       = context.Background()
)

const burstSize = 10

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

type resolvedEvent struct {
	circuitKey
	settled bool
}

type rateCounters struct {
	counters map[route.Vertex]*peerState
}

type rateCountersRequest struct {
	counters chan *rateCounters
}

type process struct {
	client lndclient
	limits *Limits
	log    *zap.SugaredLogger

	interceptChan           chan interceptEvent
	resolveChan             chan resolvedEvent
	updateLimitChan         chan updateLimitEvent
	rateCountersRequestChan chan rateCountersRequest

	identity route.Vertex
	chanMap  map[uint64]*channel
	aliasMap map[route.Vertex]string

	peerCtrls map[route.Vertex]*peerController

	burstSize int

	// Testing hook
	resolvedCallback func()
}

func NewProcess(client lndclient, log *zap.SugaredLogger, limits *Limits) *process {
	return &process{
		log:                     log,
		client:                  client,
		interceptChan:           make(chan interceptEvent),
		resolveChan:             make(chan resolvedEvent),
		updateLimitChan:         make(chan updateLimitEvent),
		rateCountersRequestChan: make(chan rateCountersRequest),
		chanMap:                 make(map[uint64]*channel),
		aliasMap:                make(map[route.Vertex]string),
		peerCtrls:               make(map[route.Vertex]*peerController),
		limits:                  limits,
		burstSize:               burstSize,
	}
}

type updateLimitEvent struct {
	limit Limit
	peer  route.Vertex
}

func (p *process) UpdateLimit(ctx context.Context, peer route.Vertex,
	limit Limit) error {

	update := updateLimitEvent{
		limit: limit,
		peer:  peer,
	}

	select {
	case p.updateLimitChan <- update:
		return nil

	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *process) Run(ctx context.Context) error {
	p.log.Info("CircuitBreaker started")

	var err error

	p.identity, err = p.client.getIdentity()
	if err != nil {
		return err
	}

	p.log.Infow("Connected to lnd node",
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

	p.log.Info("Interceptor/notification handlers registered")

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

func (p *process) getPeerController(ctx context.Context, peer route.Vertex,
	startGo func(func() error)) *peerController {

	ctrl, ok := p.peerCtrls[peer]
	if ok {
		return ctrl
	}

	// If the peer does not yet exist, initialize it with no pending htlcs.
	htlcs := make(map[circuitKey]struct{})

	return p.createPeerController(ctx, peer, startGo, htlcs)
}

func (p *process) createPeerController(ctx context.Context, peer route.Vertex,
	startGo func(func() error), htlcs map[circuitKey]struct{}) *peerController {

	// Use zero limits if not configured.
	peerCfg := p.limits.PerPeer[peer]

	logger := p.log.With(
		"peer", peer.String(),
	)

	ctrl := newPeerController(logger, peerCfg, p.burstSize, htlcs)

	startGo(func() error {
		return ctrl.run(ctx)
	})

	p.peerCtrls[peer] = ctrl

	return ctrl
}

func (p *process) eventLoop(ctx context.Context) error {
	// Create a group to attach peer goroutines to.
	group, ctx := errgroup.WithContext(ctx)
	defer func() {
		_ = group.Wait()
	}()

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
		p.createPeerController(ctx, peer, group.Go, htlcs)
	}

	for {
		select {
		case interceptEvent := <-p.interceptChan:
			chanInfo, err := p.getChanInfo(interceptEvent.channel)
			if err != nil {
				return err
			}

			ctrl := p.getPeerController(ctx, chanInfo.peer, group.Go)

			peerEvent := peerInterceptEvent{
				interceptEvent: interceptEvent,
				peerInitiated:  !chanInfo.initiator,
			}
			if err := ctrl.process(ctx, peerEvent); err != nil {
				return err
			}

		case resolvedEvent := <-p.resolveChan:
			chanInfo, err := p.getChanInfo(resolvedEvent.channel)
			if err != nil {
				return err
			}

			ctrl := p.getPeerController(ctx, chanInfo.peer, group.Go)

			if err := ctrl.resolved(ctx, resolvedEvent); err != nil {
				return err
			}

			if p.resolvedCallback != nil {
				p.resolvedCallback()
			}

		case update := <-p.updateLimitChan:
			p.limits.PerPeer[update.peer] = update.limit

			// Update specific controller if it exists.
			ctrl, ok := p.peerCtrls[update.peer]
			if ok {
				err := ctrl.updateLimit(ctx, update.limit)
				if err != nil {
					return err
				}
			}

		case req := <-p.rateCountersRequestChan:
			allCounts := make(map[route.Vertex]*peerState)
			for node, ctrl := range p.peerCtrls {
				state, err := ctrl.state(ctx)
				if err != nil {
					return err
				}

				allCounts[node] = state
			}

			req.counters <- &rateCounters{
				counters: allCounts,
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (p *process) getRateCounters(ctx context.Context) (
	map[route.Vertex]*peerState, error) {

	replyChan := make(chan *rateCounters)

	select {
	case p.rateCountersRequestChan <- rateCountersRequest{
		counters: replyChan,
	}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	select {
	case reply := <-replyChan:
		return reply.counters, nil

	case <-ctx.Done():
		return nil, ctx.Err()
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

		var settled bool
		switch event.Event.(type) {
		case *routerrpc.HtlcEvent_SettleEvent:
			settled = true

		case *routerrpc.HtlcEvent_ForwardFailEvent:
		case *routerrpc.HtlcEvent_LinkFailEvent:

		default:
			continue
		}

		select {
		case p.resolveChan <- resolvedEvent{
			settled: settled,
			circuitKey: circuitKey{
				channel: event.IncomingChannelId,
				htlc:    event.IncomingHtlcId,
			},
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
