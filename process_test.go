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

	db, cleanup := setupTestDb(t)
	defer cleanup()

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

	p := NewProcess(client, log, cfg, db)

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
		incomingCircuitKey: key,
		outgoingCircuitKey: outgoingKey,
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

	db, cleanup := setupTestDb(t)
	defer cleanup()

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

	p := NewProcess(client, log, cfg, db)
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
			incomingCircuitKey: key,
			outgoingCircuitKey: outgoingKey,
			settled:            false,
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

	db, cleanup := setupTestDb(t)
	defer cleanup()

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

	p := NewProcess(client, log, cfg, db)
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

	db, cleanup := setupTestDb(t)
	defer cleanup()

	log := zaptest.NewLogger(t).Sugar()

	cfg := &Limits{}

	p := NewProcess(client, log, cfg, db)

	// Setup quick peer refresh.
	p.peerRefreshInterval = 100 * time.Millisecond

	exit := make(chan error)
	go func() {
		exit <- p.Run(ctx)
	}()

	state, err := p.getRateCounters(ctx)
	require.NoError(t, err)
	require.Len(t, state, 3)

	// Add a new peer.
	log.Infow("Add a new peer")
	client.channels[100] = &channel{peer: route.Vertex{100}}

	// Wait for the peer to be reported.
	require.Eventually(t, func() bool {
		state, err := p.getRateCounters(ctx)
		require.NoError(t, err)

		return len(state) == 4
	}, time.Second, 100*time.Millisecond)

	cancel()
	require.ErrorIs(t, <-exit, context.Canceled)
}

func TestBlocked(t *testing.T) {
	defer Timeout()()

	db, cleanup := setupTestDb(t)
	defer cleanup()

	cfg := &Limits{
		PerPeer: map[route.Vertex]Limit{
			{2}: {
				MaxHourlyRate: 1800,
				Mode:          ModeBlock,
			},
		},
	}

	client := newLndclientMock(testChannels)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	log := zaptest.NewLogger(t).Sugar()

	p := NewProcess(client, log, cfg, db)

	exit := make(chan error)
	go func() {
		exit <- p.Run(ctx)
	}()

	var chanId uint64 = 2

	key := circuitKey{
		channel: chanId,
		htlc:    5,
	}
	interceptReq := &interceptedEvent{
		circuitKey: key,
	}

	// Htlc blocked.
	client.htlcInterceptorRequests <- interceptReq
	resp := <-client.htlcInterceptorResponses
	require.False(t, resp.resume)

	cancel()
	require.ErrorIs(t, <-exit, context.Canceled)
}

// TestChannelNotFound tests that we hit a deadlock when:
//   - We are running process.eventLoop in a go group which means that it'll cancel
//     context on error from any member of the group.
//   - Inside of process.eventLoop, we spin up a set of peerControlers in a secondary go
//     group that rely on the parent context (associated with the top level group) for
//     cancellation.
//   - There is an error in eventLoop due to an unknown channel, which prompts it to
//     return.
//   - The defer function waits for the peerController group to exit.
//   - Since the top level context will only be cancelled when eventLoop successfully
//     exits (because receiving the error will cancel ctx), we hit a deadlock:
//     -> eventLoop is waiting for all peerControllers to exit on a canceled context to
//     return an error.
//     -> the group is waiting on eventLoop to return an error to cancel context.
func TestChannelNotFound(t *testing.T) {
	client := newLndclientMock(testChannels)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	db, cleanup := setupTestDb(t)
	defer cleanup()

	log := zaptest.NewLogger(t).Sugar()

	cfg := &Limits{}

	p := NewProcess(client, log, cfg, db)

	exit := make(chan error)

	go func() {
		exit <- p.Run(ctx)
	}()

	// Next, send a htlc that is from an unknown channel.
	key := circuitKey{
		channel: 99,
		htlc:    4,
	}
	client.htlcInterceptorRequests <- &interceptedEvent{
		circuitKey: key,
	}

	// This is an inelegant way to demonstrate our hang, because the unit test will
	// take 10 seconds to run, but it shows that we're not exiting cleanly when we
	// error in eventLoop.
	select {
	case err := <-exit:
		t.Fatalf("unexpected error: %v", err)

	case <-time.After(time.Second * 10):
	}
}
