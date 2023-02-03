package main

import (
	"context"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
)

var mockIdentity = route.Vertex{1, 2, 3}

var testChannels = map[uint64]*channel{
	2: {peer: route.Vertex{2}},
	3: {peer: route.Vertex{3}, initiator: true},
}

type lndclientMock struct {
	htlcEvents               chan *resolvedEvent
	htlcInterceptorRequests  chan *interceptedEvent
	htlcInterceptorResponses chan *interceptResponse

	channels map[uint64]*channel
}

func newLndclientMock(channels map[uint64]*channel) *lndclientMock {
	return &lndclientMock{
		htlcEvents:               make(chan *resolvedEvent),
		htlcInterceptorRequests:  make(chan *interceptedEvent),
		htlcInterceptorResponses: make(chan *interceptResponse),

		channels: channels,
	}
}

func (l *lndclientMock) getIdentity() (route.Vertex, error) {
	return mockIdentity, nil
}

func (l *lndclientMock) listChannels() (map[uint64]*channel, error) {
	return l.channels, nil
}

func (l *lndclientMock) subscribeHtlcEvents(ctx context.Context) (
	htlcEventsClient, error) {

	return &htlcEventsMock{
		ctx:        ctx,
		htlcEvents: l.htlcEvents,
	}, nil
}

func (l *lndclientMock) htlcInterceptor(ctx context.Context) (
	htlcInterceptorClient, error) {

	return &htlcInterceptorMock{
		ctx:                      ctx,
		htlcInterceptorRequests:  l.htlcInterceptorRequests,
		htlcInterceptorResponses: l.htlcInterceptorResponses,
	}, nil
}

func (l *lndclientMock) getNodeAlias(key route.Vertex) (string, error) {
	return "alias-" + key.String()[:6], nil
}

func (l *lndclientMock) getPendingIncomingHtlcs(ctx context.Context, peer *route.Vertex) (
	map[route.Vertex]map[circuitKey]struct{}, error) {

	htlcs := make(map[route.Vertex]map[circuitKey]struct{})

	for _, ch := range l.channels {
		htlcs[ch.peer] = map[circuitKey]struct{}{}
	}

	return htlcs, nil
}

type htlcEventsMock struct {
	ctx context.Context //nolint:containedctx
	routerrpc.Router_SubscribeHtlcEventsClient

	htlcEvents chan *resolvedEvent
}

func (h *htlcEventsMock) recv() (*resolvedEvent, error) {
	select {
	case event := <-h.htlcEvents:
		return event, nil

	case <-h.ctx.Done():
		return nil, h.ctx.Err()
	}
}

type htlcInterceptorMock struct {
	ctx context.Context //nolint:containedctx
	routerrpc.Router_HtlcInterceptorClient

	htlcInterceptorRequests  chan *interceptedEvent
	htlcInterceptorResponses chan *interceptResponse
}

func (h *htlcInterceptorMock) send(resp *interceptResponse) error {
	select {
	case h.htlcInterceptorResponses <- resp:
		return nil

	case <-h.ctx.Done():
		return h.ctx.Err()
	}
}

func (h *htlcInterceptorMock) recv() (*interceptedEvent, error) {
	select {
	case event := <-h.htlcInterceptorRequests:
		return event, nil

	case <-h.ctx.Done():
		return nil, h.ctx.Err()
	}
}
