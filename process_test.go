package main

import (
	"context"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/stretchr/testify/require"
)

func TestProcess(t *testing.T) {
	defer Timeout()()

	t.Run("settle", func(t *testing.T) {
		testProcess(t, resolveEventSettle)
	})
	t.Run("forward fail", func(t *testing.T) {
		testProcess(t, resolveEventForwardFail)
	})
	t.Run("link fail", func(t *testing.T) {
		testProcess(t, resolveEventLinkFail)
	})
}

type resolveEvent int

const (
	resolveEventSettle resolveEvent = iota
	resolveEventForwardFail
	resolveEventLinkFail
)

func testProcess(t *testing.T, event resolveEvent) {
	p := newProcess()

	resolved := make(chan struct{})
	p.resolvedCallback = func() {
		close(resolved)
	}

	cfg := &config{
		groupConfig: groupConfig{
			MaxPendingHtlcs: 2,
		},
	}

	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exit := make(chan error)
	go func() {
		exit <- p.run(ctx, client, cfg)
	}()

	key := &routerrpc.CircuitKey{
		ChanId: 2,
		HtlcId: 5,
	}
	client.htlcInterceptorRequests <- &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey: key,
	}

	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	htlcEvent := &routerrpc.HtlcEvent{
		EventType:         routerrpc.HtlcEvent_FORWARD,
		IncomingChannelId: key.ChanId,
		IncomingHtlcId:    key.HtlcId,
	}

	switch event {
	case resolveEventForwardFail:
		htlcEvent.Event = &routerrpc.HtlcEvent_ForwardFailEvent{}

	case resolveEventLinkFail:
		htlcEvent.Event = &routerrpc.HtlcEvent_LinkFailEvent{}

	case resolveEventSettle:
		htlcEvent.Event = &routerrpc.HtlcEvent_SettleEvent{}
	}

	client.htlcEvents <- htlcEvent

	<-resolved

	cancel()
	require.NoError(t, <-exit)
}

func TestRateLimit(t *testing.T) {
	defer Timeout()()

	p := newProcess()

	cfg := &config{
		groupConfig: groupConfig{
			HtlcMinInterval: time.Minute,
			HtlcBurstSize:   2,
		},
	}

	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	exit := make(chan error)
	go func() {
		exit <- p.run(ctx, client, cfg)
	}()

	key := &routerrpc.CircuitKey{
		ChanId: 2,
		HtlcId: 5,
	}
	interceptReq := &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey: key,
	}

	// First htlc accepted.
	client.htlcInterceptorRequests <- interceptReq
	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	// Second htlc right after is also accepted because of burst size 2.
	client.htlcInterceptorRequests <- interceptReq
	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	// Third htlc again right after should be rejected.
	client.htlcInterceptorRequests <- interceptReq
	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

	cancel()
	require.NoError(t, <-exit)
}
