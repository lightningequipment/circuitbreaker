package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"

	"github.com/lightningnetwork/lnd/routing/route"
	_ "modernc.org/sqlite"
)

type Db struct {
	db *sql.DB
}

func NewDb(ctx context.Context) (*Db, error) {
	db, err := sql.Open("sqlite", "circuitbreaker.db")
	if err != nil {
		return nil, err
	}

	const initQuery string = `
	CREATE TABLE IF NOT EXISTS limits (
		peer TEXT PRIMARY KEY NOT NULL,
		htlc_max_pending INTEGER NOT NULL,
		htlc_max_hourly_rate INTEGER NOT NULL,
		mode TEXT CHECK(mode IN ('FAIL', 'QUEUE', 'QUEUE_PEER_INITIATED')) NOT NULL DEFAULT 'FAIL'
	);
	`

	if _, err := db.ExecContext(ctx, initQuery); err != nil {
		return nil, err
	}

	return &Db{
		db: db,
	}, nil
}

type Limit struct {
	MaxHourlyRate int64
	MaxPending    int64
	Mode          Mode
}

type Limits struct {
	PerPeer map[route.Vertex]Limit
}

func (d *Db) UpdateLimit(ctx context.Context, peer route.Vertex,
	limit Limit) error {

	peerHex := hex.EncodeToString(peer[:])

	if limit.MaxHourlyRate == 0 && limit.MaxPending == 0 {
		const delete string = `DELETE FROM limits WHERE peer=?;`

		_, err := d.db.ExecContext(
			ctx, delete, peerHex,
		)

		return err
	}

	const replace string = `REPLACE INTO limits(peer, htlc_max_pending, htlc_max_hourly_rate, mode) VALUES(?, ?, ?, ?);`

	_, err := d.db.ExecContext(
		ctx, replace, peerHex,
		limit.MaxPending, limit.MaxHourlyRate,
		limit.Mode.String(),
	)

	return err
}

func (d *Db) GetLimits(ctx context.Context) (*Limits, error) {
	const query string = `
	SELECT peer, htlc_max_pending, htlc_max_hourly_rate, mode from limits;`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	var limits = Limits{
		PerPeer: make(map[route.Vertex]Limit),
	}
	for rows.Next() {
		var (
			limit   Limit
			peerHex string
			modeStr string
		)
		err := rows.Scan(
			&peerHex, &limit.MaxPending, &limit.MaxHourlyRate, &modeStr,
		)
		if err != nil {
			return nil, err
		}

		switch modeStr {
		case "FAIL":
			limit.Mode = ModeFail

		case "QUEUE":
			limit.Mode = ModeQueue

		case "QUEUE_PEER_INITIATED":
			limit.Mode = ModeQueuePeerInitiated

		default:
			return nil, errors.New("unknown mode")
		}

		key, err := route.NewVertexFromStr(peerHex)
		if err != nil {
			return nil, err
		}

		limits.PerPeer[key] = limit
	}

	return &limits, nil
}
