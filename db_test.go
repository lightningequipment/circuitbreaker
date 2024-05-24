package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/require"
)

func setupTestDb(t *testing.T, fwdingHistoryLimit int) (*Db, func()) {
	file, err := os.CreateTemp("", "test_db_")
	require.NoError(t, err)

	db, err := NewDb(context.Background(), file.Name(), fwdingHistoryLimit)
	require.NoError(t, err)

	return db, func() {
		os.Remove(file.Name())
	}
}

func TestDb(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDb(t, defaultFwdHistoryLimit)
	defer cleanup()

	expectedDefaultLimit := Limit{
		MaxPending:    5,
		MaxHourlyRate: 3600,
	}

	limits, err := db.GetLimits(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedDefaultLimit, limits.Default)
	require.Len(t, limits.PerPeer, 0)

	peer := route.Vertex{1}
	limit := Limit{
		MaxHourlyRate: 1,
		MaxPending:    2,
		Mode:          ModeQueue,
	}

	require.NoError(t, db.UpdateLimit(ctx, peer, limit))

	limits, err = db.GetLimits(ctx)
	require.NoError(t, err)
	require.Equal(t, expectedDefaultLimit, limits.Default)
	require.Equal(t, map[route.Vertex]Limit{peer: limit}, limits.PerPeer)

	require.NoError(t, db.UpdateLimit(ctx, defaultNodeKey, limit))

	limits, err = db.GetLimits(ctx)
	require.NoError(t, err)
	require.Equal(t, limit, limits.Default)

	require.NoError(t, db.ClearLimit(ctx, peer))

	limits, err = db.GetLimits(ctx)
	require.NoError(t, err)
	require.Len(t, limits.PerPeer, 0)

	require.Error(t, db.ClearLimit(ctx, defaultNodeKey))

	defer db.Close()
}

func TestDbForwardingHistory(t *testing.T) {
	limit := 20

	// Create a test DB that will limit to 10 forwarding history records.
	ctx := context.Background()
	db, cleanup := setupTestDb(t, limit)
	defer cleanup()

	// Insert HTLCs just up until our limit.
	for i := 1; i < limit; i++ {
		htlc := testHtlc(uint64(i))
		require.NoError(t, db.RecordHtlcResolution(ctx, htlc))
	}

	endTime := time.Unix(100000, 0)
	fwds, err := db.ListForwardingHistory(ctx, time.Time{}, endTime)
	require.NoError(t, err)
	require.Len(t, fwds, limit-1)

	// Insert a HTLC that reaches the limit and assert that we've removed old records
	// but we still have the newest HTLC and we've culled the old one.
	limitHtlc := testHtlc(20)
	require.NoError(t, db.RecordHtlcResolution(ctx, limitHtlc))

	limitCount := limit - (limit / 10)
	fwds, err = db.ListForwardingHistory(ctx, time.Time{}, endTime)
	require.NoError(t, err)
	require.Len(t, fwds, limitCount)
	require.Equal(t, limitHtlc, fwds[len(fwds)-1])
}

func testHtlc(i uint64) *HtlcInfo {
	return &HtlcInfo{
		addTime:      time.Unix(int64(i), 0),
		resolveTime:  time.Unix(int64(i), 0),
		settled:      true,
		incomingMsat: 50,
		outgoingMsat: 45,
		incomingCircuit: circuitKey{
			channel: 1,
			htlc:    i,
		},
		outgoingCircuit: circuitKey{
			channel: 2,
			htlc:    i,
		},
		incomingPeer: route.Vertex{9, 8, 7},
		outgoingPeer: &route.Vertex{1, 2, 3},
	}
}

func TestDbNoForwardingHistory(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDb(t, 0)
	defer cleanup()

	htlc := testHtlc(1)
	require.NoError(t, db.RecordHtlcResolution(ctx, htlc))

	fwds, err := db.ListForwardingHistory(ctx, time.Time{}, time.Unix(1000000, 0))
	require.NoError(t, err)
	require.Len(t, fwds, 0)
}

func TestForwadingHistoryDelete(t *testing.T) {
	// Create a db that will store HTLCs.
	ctx := context.Background()
	db, cleanup := setupTestDb(t, 5)
	defer cleanup()

	// Write a test HTLC and assert that it's stored.
	htlc := testHtlc(1)
	require.NoError(t, db.RecordHtlcResolution(ctx, htlc))

	fwds, err := db.ListForwardingHistory(ctx, time.Time{}, time.Unix(1000000, 0))
	require.NoError(t, err)
	require.Len(t, fwds, 1)

	// Modify the db to have a zero limit on forwarding history. We don't recreate
	// the test db because it would re-create the file. Run limitHTLCRecords once
	// (as we would on NewDb) to assert that we clean up our records.
	db.fwdHistoryLimit = 0
	require.NoError(t, db.limitHTLCRecords(ctx))

	fwds, err = db.ListForwardingHistory(ctx, time.Time{}, time.Unix(1000000, 0))
	require.NoError(t, err)
	require.Len(t, fwds, 0)
}

// TestMigration4 tests the updates made to the forwarding_history table in migration 4:
// - Allow nil outgoing peer values
// - Allow non-unique outgoing_channel / index entries
func TestMigration4(t *testing.T) {
	// Create a db that will store HTLCs.
	ctx := context.Background()
	db, cleanup := setupTestDb(t, 5)
	defer cleanup()

	// Create a htlc with a nil outgoing peer.
	noOutgoingPeer := &HtlcInfo{
		addTime:      time.Unix(int64(100), 0),
		resolveTime:  time.Unix(int64(200), 0),
		settled:      true,
		incomingMsat: 50,
		outgoingMsat: 45,
		incomingCircuit: circuitKey{
			channel: 1,
			htlc:    0,
		},
		outgoingCircuit: circuitKey{
			channel: 2,
			htlc:    0,
		},
		incomingPeer: route.Vertex{9, 8, 7},
	}
	require.NoError(t, db.RecordHtlcResolution(ctx, noOutgoingPeer))

	// Create a htlc that has the same *outgoing* channel id and index.
	dupOutgoing := &HtlcInfo{
		addTime:      time.Unix(int64(101), 0),
		resolveTime:  time.Unix(int64(201), 0),
		settled:      false,
		incomingMsat: 100,
		outgoingMsat: 50,
		incomingCircuit: circuitKey{
			channel: 1,
			htlc:    1,
		},
		outgoingCircuit: circuitKey{
			channel: 2,
			htlc:    0,
		},
		incomingPeer: route.Vertex{9, 8, 7},
	}
	require.NoError(t, db.RecordHtlcResolution(ctx, dupOutgoing))

	htlcs := []*HtlcInfo{noOutgoingPeer, dupOutgoing}

	fwds, err := db.ListForwardingHistory(ctx, time.Time{}, time.Unix(1000000, 0))
	require.NoError(t, err)
	require.Equal(t, htlcs, fwds)
}
