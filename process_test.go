package main

import (
	"context"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
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
	client := newLndclientMock(testChannels)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t).Sugar()

	cfg := &Limits{
		PerPeer: map[route.Vertex]Limit{
			{2}: {
				MaxHourlyRate: 60,
				MaxPending:    1,
			},
			{3}: {
				MaxHourlyRate: 60,
				MaxPending:    1,
			},
		},
	}

	p := NewProcess(client, log, cfg)

	resolved := make(chan struct{})
	p.resolvedCallback = func() {
		close(resolved)
	}

	exit := make(chan error)
	go func() {
		exit <- p.Run(ctx)
	}()

	key := circuitKey{
		channel: 2,
		htlc:    5,
	}
	client.htlcInterceptorRequests <- &interceptedEvent{
		circuitKey: key,
	}

	resp := <-client.htlcInterceptorResponses
	require.True(t, resp.resume)

	htlcEvent := &resolvedEvent{
		circuitKey: key,
	}

	switch event {
	case resolveEventForwardFail:
		htlcEvent.settled = false

	case resolveEventLinkFail:
		htlcEvent.settled = false

	case resolveEventSettle:
		htlcEvent.settled = true
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

	cfg := &Limits{
		PerPeer: map[route.Vertex]Limit{
			{2}: {
				MaxHourlyRate: 1800,
				Mode:          mode,
			},
			{3}: {
				MaxHourlyRate: 1800,
				Mode:          mode,
			},
		},
	}

	client := newLndclientMock(testChannels)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t).Sugar()

	p := NewProcess(client, log, cfg)
	p.burstSize = 2

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

	key := circuitKey{
		channel: chanId,
		htlc:    5,
	}
	interceptReq := &interceptedEvent{
		circuitKey: key,
	}

	// First htlc accepted.
	client.htlcInterceptorRequests <- interceptReq
	resp := <-client.htlcInterceptorResponses
	require.True(t, resp.resume)

	// Second htlc right after is also accepted because of burst size 2.
	interceptReq.circuitKey.htlc++
	client.htlcInterceptorRequests <- interceptReq
	resp = <-client.htlcInterceptorResponses
	require.True(t, resp.resume)

	// Third htlc again right after should hit the rate limit.
	interceptReq.circuitKey.htlc++
	client.htlcInterceptorRequests <- interceptReq

	interceptStart := time.Now()

	resp = <-client.htlcInterceptorResponses

	if mode == ModeQueue {
		require.True(t, resp.resume)
		require.GreaterOrEqual(t, time.Since(interceptStart), time.Second)
	} else {
		require.False(t, resp.resume)

		htlcEvent := &resolvedEvent{
			circuitKey: key,
			settled:    false,
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

	cfg := &Limits{
		PerPeer: map[route.Vertex]Limit{
			{2}: {
				MaxHourlyRate: 60,
				MaxPending:    1,
				Mode:          mode,
			},
			{3}: {
				MaxHourlyRate: 60,
				MaxPending:    1,
				Mode:          mode,
			},
		},
	}

	client := newLndclientMock(testChannels)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t).Sugar()

	p := NewProcess(client, log, cfg)
	p.burstSize = 2

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

	key := circuitKey{
		channel: chanId,
		htlc:    5,
	}
	interceptReq := &interceptedEvent{
		circuitKey: key,
	}

	// First htlc accepted.
	client.htlcInterceptorRequests <- interceptReq
	resp := <-client.htlcInterceptorResponses
	require.True(t, resp.resume)

	// Second htlc should be hitting the max pending htlcs limit.
	interceptReq.circuitKey.htlc++
	client.htlcInterceptorRequests <- interceptReq

	if mode == ModeQueue {
		select {
		case <-client.htlcInterceptorResponses:
			require.Fail(t, "unexpected response")

		case <-time.After(time.Second):
		}
	} else {
		resp = <-client.htlcInterceptorResponses
		require.False(t, resp.resume)
	}

	cancel()
	require.ErrorIs(t, <-exit, context.Canceled)
}

func TestNewPeer(t *testing.T) {
	// Initialize lnd with test channels.
	client := newLndclientMock(testChannels)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t).Sugar()

	cfg := &Limits{}

	p := NewProcess(client, log, cfg)

	// Setup quick peer refresh.
	p.peerRefreshInterval = 100 * time.Millisecond

	exit := make(chan error)
	go func() {
		exit <- p.Run(ctx)
	}()

	state, err := p.getRateCounters(ctx)
	require.NoError(t, err)
	require.Len(t, state, 2)

	// Add a new peer.
	log.Infow("Add a new peer")
	client.channels[100] = &channel{peer: route.Vertex{100}}

	// Wait for the peer to be reported.
	require.Eventually(t, func() bool {
		state, err := p.getRateCounters(ctx)
		require.NoError(t, err)

		return len(state) == 3
	}, time.Second, 100*time.Millisecond)

	cancel()
	require.ErrorIs(t, <-exit, context.Canceled)
}
