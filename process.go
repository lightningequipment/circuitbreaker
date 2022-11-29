package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

var (
	rpcTimeout = 10 * time.Second
	ctxb       = context.Background()
)

type lndclient interface {
	getIdentity() (route.Vertex, error)

	getChanInfo(channel uint64) (*channelEdge, error)

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

	interceptChan chan interceptEvent
	resolveChan   chan circuitKey

	identity  route.Vertex
	pubkeyMap map[uint64]route.Vertex
	aliasMap  map[route.Vertex]string

	limiters map[route.Vertex]*rate.Limiter

	// Testing hook
	resolvedCallback func()
}

func newProcess() *process {
	return &process{
		interceptChan: make(chan interceptEvent),
		resolveChan:   make(chan circuitKey),
		pubkeyMap:     make(map[uint64]route.Vertex),
		aliasMap:      make(map[route.Vertex]string),
		limiters:      make(map[route.Vertex]*rate.Limiter),
	}
}

func (p *process) run(ctx context.Context, client lndclient, cfg *config) error {
	log.Info("CircuitBreaker started")

	p.client = client

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
		return p.eventLoop(ctx, cfg)
	})

	return group.Wait()
}

type peerInfo struct {
	htlcs map[circuitKey]struct{}
}

func (p *process) rateLimit(peer route.Vertex, cfg *groupConfig) bool {
	// Skip if no interval set.
	if cfg.HtlcMinInterval == 0 {
		return false
	}

	// Get or create rate limiter with config that applies to this peer.
	limiter, ok := p.limiters[peer]
	if !ok {
		limiter = rate.NewLimiter(
			rate.Every(cfg.HtlcMinInterval),
			cfg.HtlcBurstSize,
		)
		p.limiters[peer] = limiter
	}

	// Apply rate limit.
	return !limiter.Allow()
}

func (p *process) eventLoop(ctx context.Context, cfg *config) error {
	pendingHtlcs := make(map[route.Vertex]*peerInfo)

	// Initialize pending htlcs map with currently pending htlcs.
	htlcs, err := p.client.getPendingIncomingHtlcs(ctx)
	if err != nil {
		return err
	}
	for h := range htlcs {
		peer, err := p.getPubKey(h.channel)
		if err != nil {
			return err
		}

		pending, ok := pendingHtlcs[peer]
		if !ok {
			pending = &peerInfo{
				htlcs: make(map[circuitKey]struct{}),
			}
			pendingHtlcs[peer] = pending
		}

		pending.htlcs[h] = struct{}{}

		log.Infow("Initial pending htlc",
			"peer", peer, "channel", h.channel, "htlc", h.htlc,
		)
	}

	for {
		select {
		case interceptEvent := <-p.interceptChan:
			peer, err := p.getPubKey(interceptEvent.channel)
			if err != nil {
				return err
			}

			alias := p.getNodeAlias(peer)

			logger := log.With(
				"channel", interceptEvent.channel,
				"htlc", interceptEvent.htlc,
				"peer_alias", alias,
				"peer", peer.String(),
			)

			peerCfg := cfg.forPeer(peer)

			if p.rateLimit(peer, peerCfg) {
				logger.Infow("Rejecting htlc because of rate limit",
					"htlc_min_interval", peerCfg.HtlcMinInterval,
					"htlc_burst_size", peerCfg.HtlcBurstSize,
				)

				if err := interceptEvent.resume(false); err != nil {
					return err
				}

				continue
			}

			// Retrieve list of pending htlcs for this peer.
			pending, ok := pendingHtlcs[peer]
			if !ok {
				pending = &peerInfo{
					htlcs: make(map[circuitKey]struct{}),
				}
				pendingHtlcs[peer] = pending
			}

			// If htlc is new, check the max pending htlcs limit.
			if _, exists := pending.htlcs[interceptEvent.circuitKey]; !exists {
				maxPending := peerCfg.MaxPendingHtlcs

				if maxPending > 0 && len(pending.htlcs) >= maxPending {
					logger.Infow("Rejecting htlc",
						"pending_htlcs", len(pending.htlcs),
						"max_pending_htlcs", maxPending,
					)

					if err := interceptEvent.resume(false); err != nil {
						return err
					}

					continue
				}

				pending.htlcs[interceptEvent.circuitKey] = struct{}{}
			}

			logger.Infow("Forwarding htlc",
				"pending_htlcs", len(pending.htlcs),
			)

			if err := interceptEvent.resume(true); err != nil {
				return err
			}

		case resolvedKey := <-p.resolveChan:
			peer, err := p.getPubKey(resolvedKey.channel)
			if err != nil {
				return err
			}

			pending, ok := pendingHtlcs[peer]
			if !ok {
				continue
			}

			_, ok = pending.htlcs[resolvedKey]
			if !ok {
				continue
			}

			delete(pending.htlcs, resolvedKey)

			log.Infow("Resolving htlc",
				"channel", resolvedKey.channel,
				"htlc", resolvedKey.htlc,
				"peer_alias", p.getNodeAlias(peer),
				"peer", peer.String(),
				"pending_htlcs", len(pending.htlcs),
			)

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

func (p *process) getPubKey(channel uint64) (route.Vertex, error) {
	pubkey, ok := p.pubkeyMap[channel]
	if ok {
		return pubkey, nil
	}

	edge, err := p.client.getChanInfo(channel)
	if err != nil {
		return route.Vertex{}, err
	}

	var remotePubkey route.Vertex
	switch {
	case edge.node1Pub == p.identity:
		remotePubkey = edge.node2Pub

	case edge.node2Pub == p.identity:
		remotePubkey = edge.node1Pub

	default:
		return route.Vertex{}, errors.New("identity not found in chan info")
	}

	p.pubkeyMap[channel] = remotePubkey

	return remotePubkey, nil
}
