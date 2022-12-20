package circuitbreaker

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
	db     *Db
	limits *Limits
	log    *zap.SugaredLogger

	interceptChan   chan interceptEvent
	resolveChan     chan circuitKey
	updateLimitChan chan updateLimitEvent

	identity route.Vertex
	chanMap  map[uint64]*channel
	aliasMap map[route.Vertex]string

	peerCtrls map[route.Vertex]*peerController

	// Testing hook
	resolvedCallback func()
}

func NewProcess(client lndclient, log *zap.SugaredLogger, db *Db) *process {
	return &process{
		log:             log,
		client:          client,
		interceptChan:   make(chan interceptEvent),
		resolveChan:     make(chan circuitKey),
		updateLimitChan: make(chan updateLimitEvent),
		chanMap:         make(map[uint64]*channel),
		aliasMap:        make(map[route.Vertex]string),
		db:              db,
		peerCtrls:       make(map[route.Vertex]*peerController),
	}
}

type updateLimitEvent struct {
	limit Limit
	peer  *route.Vertex
}

func (p *process) UpdateLimit(ctx context.Context, peer *route.Vertex,
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

	p.limits, err = p.db.GetLimits(ctx)
	if err != nil {
		return err
	}

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

	peerCfg, ok := p.limits.PerPeer[peer]
	if !ok {
		peerCfg = p.limits.Global
	}

	alias := p.getNodeAlias(peer)

	logger := p.log.With(
		"peer_alias", alias,
		"peer", peer.String(),
	)

	ctrl := newPeerController(logger, peerCfg, htlcs)

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

		case resolvedKey := <-p.resolveChan:
			chanInfo, err := p.getChanInfo(resolvedKey.channel)
			if err != nil {
				return err
			}

			ctrl := p.getPeerController(ctx, chanInfo.peer, group.Go)

			if err := ctrl.resolved(ctx, resolvedKey); err != nil {
				return err
			}

			if p.resolvedCallback != nil {
				p.resolvedCallback()
			}

		case update := <-p.updateLimitChan:
			if update.peer == nil {
				p.limits.Global = update.limit

				// Update all controllers that have no specific limit.
				for node, ctrl := range p.peerCtrls {
					_, ok := p.limits.PerPeer[node]
					if ok {
						continue
					}

					err := ctrl.updateLimit(ctx, update.limit)
					if err != nil {
						return err
					}
				}
			} else {
				p.limits.PerPeer[*update.peer] = update.limit

				// Update specific controller if it exists.
				ctrl, ok := p.peerCtrls[*update.peer]
				if ok {
					err := ctrl.updateLimit(ctx, update.limit)
					if err != nil {
						return err
					}
				}
			}

			err := p.db.SetLimit(ctx, update.peer, update.limit)
			if err != nil {
				return err
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
		p.log.Warnw("cannot get node alias",
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
