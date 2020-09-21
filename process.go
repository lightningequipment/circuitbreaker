package main

import (
	"context"
	"errors"
	"time"

	"github.com/lightninglabs/protobuf-hex-display/jsonpb"
	"github.com/lightninglabs/protobuf-hex-display/proto"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"go.uber.org/zap"
)

const maxPending = 1

var (
	rpcTimeout = 10 * time.Second
	ctxb       = context.Background()
)

type circuitKey struct {
	channel uint64
	htlc    uint64
}

type interceptEvent struct {
	circuitKey
	resume chan bool
}

type process struct {
	client *lndclient

	interceptChan chan interceptEvent
	resolveChan   chan circuitKey

	identity  route.Vertex
	pubkeyMap map[uint64]route.Vertex
}

func newProcess() *process {
	return &process{
		interceptChan: make(chan interceptEvent),
		resolveChan:   make(chan circuitKey),
		pubkeyMap:     make(map[uint64]route.Vertex),
	}
}

func (p *process) run(client *lndclient, cfg *config) error {
	log.Info("CircuitBreaker started")

	p.client = client

	var err error
	p.identity, err = p.client.getIdentity()
	if err != nil {
		return err
	}

	log.Info("Connected to lnd node",
		zap.String("pubkey", p.identity.String()))

	stream, err := p.client.router.SubscribeHtlcEvents(
		ctxb, &routerrpc.SubscribeHtlcEventsRequest{},
	)
	if err != nil {
		return err
	}

	interceptor, err := p.client.router.HtlcInterceptor(ctxb)
	if err != nil {
		return err
	}

	log.Info("Interceptor/notification handlers registered")

	go func() {
		err := p.processHtlcEvents(stream)
		if err != nil {
			log.Error("htlc events error", zap.Error(err))
		}
	}()

	go func() {
		err := p.processInterceptor(interceptor)
		if err != nil {
			log.Error("interceptor error", zap.Error(err))
		}
	}()

	return p.eventLoop(cfg)
}

func (p *process) eventLoop(cfg *config) error {
	pendingHtlcs := make(map[route.Vertex]map[circuitKey]struct{})
	for {
		select {
		case interceptEvent := <-p.interceptChan:
			peer, err := p.getPubKey(interceptEvent.channel)
			if err != nil {
				return err
			}

			pending, ok := pendingHtlcs[peer]
			if !ok {
				pending = make(map[circuitKey]struct{})
				pendingHtlcs[peer] = pending
			}

			maxPending, ok := cfg.MaxPendingHtlcsPerPeer[peer]
			if !ok {
				maxPending = cfg.MaxPendingHtlcs
			}

			if len(pending) >= maxPending {
				log.Info("Rejecting htlc",
					zap.Uint64("channel", interceptEvent.channel),
					zap.Uint64("htlc", interceptEvent.htlc),
					zap.String("peer", peer.String()),
					zap.Int("pending_htlcs", len(pending)),
					zap.Int("max_pending_htlcs", maxPending),
				)

				interceptEvent.resume <- false
				continue
			}

			pending[interceptEvent.circuitKey] = struct{}{}

			log.Info("Forwarding htlc",
				zap.Uint64("channel", interceptEvent.channel),
				zap.Uint64("htlc", interceptEvent.htlc),
				zap.String("peer", peer.String()),
				zap.Int("pending_htlcs", len(pending)),
				zap.Int("max_pending_htlcs", maxPending),
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

			if _, ok := pending[resolvedKey]; !ok {
				continue
			}

			delete(pending, resolvedKey)

			log.Info("Resolving htlc",
				zap.Uint64("channel", resolvedKey.channel),
				zap.Uint64("htlc", resolvedKey.htlc),
				zap.String("peer", peer.String()),
				zap.Int("pending_htlcs", len(pending)))
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
			resume: resumeChan,
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

func jsonToString(resp proto.Message) (string, error) {
	jsonMarshaler := &jsonpb.Marshaler{
		EmitDefaults: true,
		OrigName:     true,
		Indent:       "    ",
	}

	jsonStr, err := jsonMarshaler.MarshalToString(resp)
	if err != nil {
		return "", err
	}

	return jsonStr, nil
}
