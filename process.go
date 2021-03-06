package main

import (
	"context"
	"errors"
	"time"

	"github.com/lightninglabs/protobuf-hex-display/jsonpb"
	"github.com/lightninglabs/protobuf-hex-display/proto"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
)

const maxPending = 1

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
}

func newProcess() *process {
	return &process{
		interceptChan: make(chan interceptEvent),
		resolveChan:   make(chan circuitKey),
		pubkeyMap:     make(map[uint64]route.Vertex),
		aliasMap:      make(map[route.Vertex]string),
	}
}

func (p *process) run(ctx context.Context, client lndclient, cfg *config) error {
	log.Info("CircuitBreaker started")
	log.Infow("Hold fee",
		"base", cfg.BaseSatPerHr,
		"rate", float64(cfg.RatePpmPerHr)/1e6,
		"reporting_interval", cfg.ReportingInterval,
	)

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

type holdInfo struct {
	fwdTime   time.Time
	valueMsat int64
}

type peerInfo struct {
	htlcs map[circuitKey]*holdInfo

	totalHoldFees    int64
	intervalHoldFees int64
}

func (p *process) eventLoop(ctx context.Context, cfg *config) error {
	pendingHtlcs := make(map[route.Vertex]*peerInfo)

	intervalNs := int64(cfg.ReportingInterval)
	nextReport := time.Unix(0, (time.Now().UnixNano()/intervalNs+1)*
		intervalNs)

	log.Infow("First hold fees report scheduled", "next_report_time", nextReport)

	for {
		timeToReport := nextReport.Sub(time.Now())

		select {
		case interceptEvent := <-p.interceptChan:
			peer, err := p.getPubKey(interceptEvent.channel)
			if err != nil {
				return err
			}

			alias := p.getNodeAlias(peer)

			pending, ok := pendingHtlcs[peer]
			if !ok {
				pending = &peerInfo{
					htlcs: make(map[circuitKey]*holdInfo),
				}
				pendingHtlcs[peer] = pending
			}

			maxPending, ok := cfg.MaxPendingHtlcsPerPeer[peer]
			if !ok {
				maxPending = cfg.MaxPendingHtlcs
			}

			if len(pending.htlcs) >= maxPending {
				log.Infow("Rejecting htlc",
					"channel", interceptEvent.channel,
					"htlc", interceptEvent.htlc,
					"peer_alias", alias,
					"peer", peer.String(),
					"pending_htlcs", len(pending.htlcs),
					"max_pending_htlcs", maxPending,
				)

				interceptEvent.resume <- false
				continue
			}

			pending.htlcs[interceptEvent.circuitKey] = &holdInfo{
				fwdTime:   time.Now(),
				valueMsat: interceptEvent.valueMsat,
			}

			log.Infow("Forwarding htlc",
				"channel", interceptEvent.channel,
				"htlc", interceptEvent.htlc,
				"peer_alias", alias,
				"peer", peer.String(),
				"pending_htlcs", len(pending.htlcs),
				"max_pending_htlcs", maxPending,
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

			info, ok := pending.htlcs[resolvedKey]
			if !ok {
				continue
			}

			delete(pending.htlcs, resolvedKey)

			holdTime := time.Since(info.fwdTime)

			holdFeeMsat := int64((1000*float64(cfg.BaseSatPerHr) +
				float64(info.valueMsat)*float64(cfg.RatePpmPerHr)/1e6) *
				holdTime.Hours())

			pending.totalHoldFees += holdFeeMsat
			pending.intervalHoldFees += holdFeeMsat

			log.Infow("Resolving htlc",
				"channel", resolvedKey.channel,
				"htlc", resolvedKey.htlc,
				"peer_alias", p.getNodeAlias(peer),
				"peer", peer.String(),
				"pending_htlcs", len(pending.htlcs),
				"hold_time", holdTime,
				"hold_fee_msat", holdFeeMsat)

		case <-time.After(timeToReport):
			changedPeers := []route.Vertex{}
			for key, info := range pendingHtlcs {
				if info.intervalHoldFees > 0 {
					changedPeers = append(
						changedPeers, key,
					)
				}
			}

			nextReport = nextReport.Add(cfg.ReportingInterval)

			if len(changedPeers) == 0 {
				log.Infow("No hold fees to report",
					"next_report_time", nextReport)
			} else {
				log.Infow("Hold fees report",
					"next_report_time", nextReport)

				for _, key := range changedPeers {
					log.Infow("Report",
						"peer_alias", p.getNodeAlias(key),
						"peer", key,
						"total_fees_msat", pendingHtlcs[key].totalHoldFees,
						"interval_fees_msat", pendingHtlcs[key].intervalHoldFees,
					)

					pendingHtlcs[key].intervalHoldFees = 0
				}
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
