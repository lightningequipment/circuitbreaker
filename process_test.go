package main

import (
	"context"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
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
	cfg := &Config{
		GroupConfig: GroupConfig{
			MaxPendingHtlcs: 2,
		},
	}

	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log, err := zap.NewDevelopment()
	require.NoError(t, err)

	p := NewProcess(client, log.Sugar(), cfg)

	resolved := make(chan struct{})
	p.resolvedCallback = func() {
		close(resolved)
	}

	exit := make(chan error)
	go func() {
		exit <- p.Run(ctx)
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
	require.ErrorIs(t, <-exit, context.Canceled)
}

func TestLimits(t *testing.T) {
	for _, mode := range []Mode{ModeFail, ModeQueue, ModeQueuePeerInitiated} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Run("rate limit", func(t *testing.T) { testRateLimit(t, mode) })
			t.Run("max pending", func(t *testing.T) { testMaxPending(t, mode) })
		})
	}
}

func testRateLimit(t *testing.T, mode Mode) {
	defer Timeout()()

	cfg := &Config{
		GroupConfig: GroupConfig{
			HtlcMinInterval: 2 * time.Second,
			HtlcBurstSize:   2,
			Mode:            mode,
		},
	}

	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log, err := zap.NewDevelopment()
	require.NoError(t, err)

	p := NewProcess(client, log.Sugar(), cfg)

	exit := make(chan error)
	go func() {
		exit <- p.Run(ctx)
	}()

	var chanId uint64 = 2
	if mode == ModeQueuePeerInitiated {
		// We are the initiator of the channel. Not queueing is expected in this
		// mode.
		chanId = 3
	}

	key := &routerrpc.CircuitKey{
		ChanId: chanId,
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
	interceptReq.IncomingCircuitKey.HtlcId++
	client.htlcInterceptorRequests <- interceptReq
	resp = <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	// Third htlc again right after should hit the rate limit.
	interceptReq.IncomingCircuitKey.HtlcId++
	client.htlcInterceptorRequests <- interceptReq

	interceptStart := time.Now()

	resp = <-client.htlcInterceptorResponses

	if mode == ModeQueue {
		require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)
		require.GreaterOrEqual(t, time.Since(interceptStart), time.Second)
	} else {
		require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)

		htlcEvent := &routerrpc.HtlcEvent{
			EventType:         routerrpc.HtlcEvent_FORWARD,
			IncomingChannelId: key.ChanId,
			IncomingHtlcId:    key.HtlcId,
			Event:             &routerrpc.HtlcEvent_ForwardFailEvent{},
		}

		client.htlcEvents <- htlcEvent

		// Allow some time for the peer controller to process the failed forward
		// event.
		time.Sleep(time.Second)
	}

	cancel()
	require.ErrorIs(t, <-exit, context.Canceled)
}

func testMaxPending(t *testing.T, mode Mode) {
	defer Timeout()()

	cfg := &Config{
		GroupConfig: GroupConfig{
			HtlcMinInterval: time.Minute,
			HtlcBurstSize:   2,
			MaxPendingHtlcs: 1,
			Mode:            mode,
		},
	}

	client := newLndclientMock()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log, err := zap.NewDevelopment()
	require.NoError(t, err)

	p := NewProcess(client, log.Sugar(), cfg)

	exit := make(chan error)
	go func() {
		exit <- p.Run(ctx)
	}()

	var chanId uint64 = 2
	if mode == ModeQueuePeerInitiated {
		// We are the initiator of the channel. Not queueing is expected in this
		// mode.
		chanId = 3
	}

	key := &routerrpc.CircuitKey{
		ChanId: chanId,
		HtlcId: 5,
	}
	interceptReq := &routerrpc.ForwardHtlcInterceptRequest{
		IncomingCircuitKey: key,
	}

	// First htlc accepted.
	client.htlcInterceptorRequests <- interceptReq
	resp := <-client.htlcInterceptorResponses
	require.Equal(t, routerrpc.ResolveHoldForwardAction_RESUME, resp.Action)

	// Second htlc should be hitting the max pending htlcs limit.
	interceptReq.IncomingCircuitKey.HtlcId++
	client.htlcInterceptorRequests <- interceptReq

	if mode == ModeQueue {
		select {
		case <-client.htlcInterceptorResponses:
			require.Fail(t, "unexpected response")

		case <-time.After(time.Second):
		}
	} else {
		resp = <-client.htlcInterceptorResponses
		require.Equal(t, routerrpc.ResolveHoldForwardAction_FAIL, resp.Action)
	}

	cancel()
	require.ErrorIs(t, <-exit, context.Canceled)
}
