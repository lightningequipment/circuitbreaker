package main

import (
	"context"
	"os"
	"testing"

	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/stretchr/testify/require"
)

func setupTestDb(t *testing.T) (*Db, func()) {
	file, err := os.CreateTemp("", "test_db_")
	require.NoError(t, err)

	db, err := NewDb(file.Name())
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
