package main

import (
	"context"
	"errors"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
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
}

type circuitKey struct {
	channel uint64
	htlc    uint64
}

type interceptEvent struct {
	circuitKey
	valueMsat int64
	resume    chan bool
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

	stream, err := p.client.subscribeHtlcEvents(
		ctxb, &routerrpc.SubscribeHtlcEventsRequest{},
	)
	if err != nil {
		return err
	}

	interceptor, err := p.client.htlcInterceptor(ctxb)
	if err != nil {
		return err
	}

	log.Info("Interceptor/notification handlers registered")

	go func() {
		err := p.processHtlcEvents(stream)
		if err != nil {
			log.Errorw("htlc events error",
				"err", err)
		}
	}()

	go func() {
		err := p.processInterceptor(interceptor)
		if err != nil {
			log.Errorw("interceptor error",
				"err", err)
		}
	}()

	return p.eventLoop(ctx, cfg)
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

				interceptEvent.resume <- false
				continue
			}

			// Apply max htlc limit.
			pending, ok := pendingHtlcs[peer]
			if !ok {
				pending = &peerInfo{
					htlcs: make(map[circuitKey]struct{}),
				}
				pendingHtlcs[peer] = pending
			}

			maxPending := peerCfg.MaxPendingHtlcs

			if maxPending > 0 && len(pending.htlcs) >= maxPending {
				logger.Infow("Rejecting htlc",
					"pending_htlcs", len(pending.htlcs),
					"max_pending_htlcs", maxPending,
				)

				interceptEvent.resume <- false
				continue
			}

			pending.htlcs[interceptEvent.circuitKey] = struct{}{}

			logger.Infow("Forwarding htlc",
				"pending_htlcs", len(pending.htlcs),
			)

			interceptEvent.resume <- true

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
			log.Info("Exit")

			return nil
		}
	}
}

func (p *process) processHtlcEvents(stream routerrpc.Router_SubscribeHtlcEventsClient) error {
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
			p.resolveChan <- circuitKey{
				channel: event.IncomingChannelId,
				htlc:    event.IncomingHtlcId,
			}

		case *routerrpc.HtlcEvent_ForwardFailEvent:
			p.resolveChan <- circuitKey{
				channel: event.IncomingChannelId,
				htlc:    event.IncomingHtlcId,
			}
			
		case *routerrpc.HtlcEvent_LinkFailEvent:
			p.resolveChan <- circuitKey{
				channel: event.IncomingChannelId,
				htlc:    event.IncomingHtlcId,
			}
		}
	}
}

func (p *process) processInterceptor(interceptor routerrpc.Router_HtlcInterceptorClient) error {
	for {
		event, err := interceptor.Recv()
		if err != nil {
			return err
		}

		resumeChan := make(chan bool)

		p.interceptChan <- interceptEvent{
			circuitKey: circuitKey{
				channel: event.IncomingCircuitKey.ChanId,
				htlc:    event.IncomingCircuitKey.HtlcId,
			},
			valueMsat: int64(event.OutgoingAmountMsat),
			resume:    resumeChan,
		}

		resume, ok := <-resumeChan
		if !ok {
			return errors.New("resume channel closed")
		}

		response := &routerrpc.ForwardHtlcInterceptResponse{
			IncomingCircuitKey: event.IncomingCircuitKey,
		}
		if resume {
			response.Action = routerrpc.ResolveHoldForwardAction_RESUME
		} else {
			response.Action = routerrpc.ResolveHoldForwardAction_FAIL
		}

		err = interceptor.Send(response)
		if err != nil {
			return err
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
