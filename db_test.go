package main

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/require"
)

func setupTestDb(t *testing.T, dbOpts ...func(*Db)) (*Db, func()) {
	file, err := os.CreateTemp("", "test_db_")
	require.NoError(t, err)

	db, err := NewDb(file.Name(), dbOpts...)
	require.NoError(t, err)

	return db, func() {
		os.Remove(file.Name())
	}
}

func TestDb(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestDb(t)
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

func dbWithCustomForwardingHistoryLimit(limit int) func(d *Db) {
	return func(d *Db) {
		d.fwdHistoryLimit = limit
	}
}

func TestDbForwardingHistory(t *testing.T) {
	limit := 20

	// Create a test DB that will limit to 10 forwarding history records.
	ctx := context.Background()
	db, cleanup := setupTestDb(t, dbWithCustomForwardingHistoryLimit(limit))
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
	}
}
