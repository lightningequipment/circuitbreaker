package main

import (
	"context"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
)

var mockIdentity = route.Vertex{1, 2, 3}

type lndclientMock struct {
	htlcEvents               chan *routerrpc.HtlcEvent
	htlcInterceptorRequests  chan *routerrpc.ForwardHtlcInterceptRequest
	htlcInterceptorResponses chan *routerrpc.ForwardHtlcInterceptResponse
}

func newLndclientMock() *lndclientMock {
	return &lndclientMock{
		htlcEvents:               make(chan *routerrpc.HtlcEvent),
		htlcInterceptorRequests:  make(chan *routerrpc.ForwardHtlcInterceptRequest),
		htlcInterceptorResponses: make(chan *routerrpc.ForwardHtlcInterceptResponse),
	}
}

func (l *lndclientMock) getIdentity() (route.Vertex, error) {
	return mockIdentity, nil
}

func (l *lndclientMock) listChannels() (map[uint64]*channel, error) {
	return map[uint64]*channel{
		2: {peer: route.Vertex{2}},
		3: {peer: route.Vertex{3}, initiator: true},
	}, nil
}

func (l *lndclientMock) subscribeHtlcEvents(ctx context.Context,
	in *routerrpc.SubscribeHtlcEventsRequest) (
	routerrpc.Router_SubscribeHtlcEventsClient, error) {

	return &htlcEventsMock{
		ctx:        ctx,
		htlcEvents: l.htlcEvents,
	}, nil
}

func (l *lndclientMock) htlcInterceptor(ctx context.Context) (
	routerrpc.Router_HtlcInterceptorClient, error) {

	return &htlcInterceptorMock{
		ctx:                      ctx,
		htlcInterceptorRequests:  l.htlcInterceptorRequests,
		htlcInterceptorResponses: l.htlcInterceptorResponses,
	}, nil
}

func (l *lndclientMock) getNodeAlias(key route.Vertex) (string, error) {
	return "alias-" + key.String()[:6], nil
}

func (l *lndclientMock) getPendingIncomingHtlcs(ctx context.Context) (
	map[circuitKey]struct{}, error) {

	htlcs := make(map[circuitKey]struct{})

	return htlcs, nil
}

type htlcEventsMock struct {
	ctx context.Context //nolint:containedctx
	routerrpc.Router_SubscribeHtlcEventsClient

	htlcEvents chan *routerrpc.HtlcEvent
}

func (h *htlcEventsMock) Recv() (*routerrpc.HtlcEvent, error) {
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

	htlcInterceptorRequests  chan *routerrpc.ForwardHtlcInterceptRequest
	htlcInterceptorResponses chan *routerrpc.ForwardHtlcInterceptResponse
}

func (h *htlcInterceptorMock) Send(resp *routerrpc.ForwardHtlcInterceptResponse) error {
	select {
	case h.htlcInterceptorResponses <- resp:
		return nil

	case <-h.ctx.Done():
		return h.ctx.Err()
	}
}

func (h *htlcInterceptorMock) Recv() (*routerrpc.ForwardHtlcInterceptRequest, error) {
	select {
	case event := <-h.htlcInterceptorRequests:
		return event, nil

	case <-h.ctx.Done():
		return nil, h.ctx.Err()
	}
}
