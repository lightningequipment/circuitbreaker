package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
	migrate "github.com/rubenv/sql-migrate"
	_ "modernc.org/sqlite"
)

var migrations = &migrate.MemoryMigrationSource{
	Migrations: []*migrate.Migration{
		{
			Id: "1",
			Up: []string{
				`
				CREATE TABLE IF NOT EXISTS limits (
					peer TEXT PRIMARY KEY NOT NULL,
					htlc_max_pending INTEGER NOT NULL,
					htlc_max_hourly_rate INTEGER NOT NULL,
					mode TEXT CHECK(mode IN ('FAIL', 'QUEUE', 'QUEUE_PEER_INITIATED')) NOT NULL DEFAULT 'FAIL'
				);
				
				INSERT OR IGNORE INTO limits(peer, htlc_max_pending, htlc_max_hourly_rate) 
				VALUES('000000000000000000000000000000000000000000000000000000000000000000', 5, 3600);
				`,
			},
		},
		{
			Id: "2",
			Up: []string{
				`
				ALTER TABLE limits RENAME TO limits_old;

				CREATE TABLE IF NOT EXISTS limits (
					peer TEXT PRIMARY KEY NOT NULL,
					htlc_max_pending INTEGER NOT NULL,
					htlc_max_hourly_rate INTEGER NOT NULL,
					mode TEXT CHECK(mode IN ('FAIL', 'QUEUE', 'QUEUE_PEER_INITIATED', 'BLOCK')) NOT NULL DEFAULT 'FAIL'
				);

				INSERT INTO limits(peer, htlc_max_pending, htlc_max_hourly_rate, mode)
					SELECT peer, htlc_max_pending, htlc_max_hourly_rate, mode FROM limits_old;

				DROP TABLE limits_old;
				`,
			},
		},
		{
			Id: "3",
			Up: []string{
				`CREATE TABLE IF NOT EXISTS forwarding_history (
                                        add_time TIMESTAMP NOT NULL,
                                        resolved_time TIMESTAMP NOT NULL,
                                        settled BOOLEAN NOT NULL,
                                        incoming_amt_msat INTEGER NOT NULL CHECK (incoming_amt_msat > 0),
                                        outgoing_amt_msat INTEGER NOT NULL CHECK (outgoing_amt_msat > 0),
                                        incoming_peer TEXT NOT NULL,
                                        incoming_channel INTEGER NOT NULL,
                                        incoming_htlc_index INTEGER NOT NULL,
                                        outgoing_peer TEXT NOT NULL,
                                        outgoing_channel INTEGER NOT NULL,
                                        outgoing_htlc_index INTEGER NOT NULL,
                                        
                                        CONSTRAINT unique_incoming_circuit UNIQUE (incoming_channel, incoming_htlc_index),
                                        CONSTRAINT unique_outgoing_circuit UNIQUE (outgoing_channel, outgoing_htlc_index)
                                );`,
				`CREATE INDEX add_time_index ON forwarding_history (add_time);`,
			},
		},
	},
}

var defaultNodeKey = route.Vertex{}

type Db struct {
	db *sql.DB
}

func NewDb(dbPath string) (*Db, error) {
	const busyTimeoutMs = 5000

	dsn := dbPath + fmt.Sprintf("?_pragma=busy_timeout=%d", busyTimeoutMs)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	n, err := migrate.Exec(db, "sqlite3", migrations, migrate.Up)
	if err != nil {
		return nil, fmt.Errorf("migration error: %w", err)
	}
	if n > 0 {
		log.Infow("Applied migrations", "count", n)
	}

	return &Db{
		db: db,
	}, nil
}

func (d *Db) Close() error {
	return d.db.Close()
}

type Limit struct {
	MaxHourlyRate int64
	MaxPending    int64
	Mode          Mode
}

type Limits struct {
	Default Limit
	PerPeer map[route.Vertex]Limit
}

func (d *Db) UpdateLimit(ctx context.Context, peer route.Vertex,
	limit Limit) error {

	peerHex := hex.EncodeToString(peer[:])

	const replace string = `REPLACE INTO limits(peer, htlc_max_pending, htlc_max_hourly_rate, mode) VALUES(?, ?, ?, ?);`

	_, err := d.db.ExecContext(
		ctx, replace, peerHex,
		limit.MaxPending, limit.MaxHourlyRate,
		limit.Mode.String(),
	)

	return err
}

func (d *Db) ClearLimit(ctx context.Context, peer route.Vertex) error {
	if peer == defaultNodeKey {
		return errors.New("cannot clear default limit")
	}

	const query string = `DELETE FROM limits WHERE peer = ?;`

	_, err := d.db.ExecContext(
		ctx, query, hex.EncodeToString(peer[:]),
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

		case "BLOCK":
			limit.Mode = ModeBlock

		default:
			return nil, errors.New("unknown mode")
		}

		key, err := route.NewVertexFromStr(peerHex)
		if err != nil {
			return nil, err
		}

		if key == defaultNodeKey {
			limits.Default = limit
		} else {
			limits.PerPeer[key] = limit
		}
	}

	return &limits, nil
}

type HtlcInfo struct {
	addTime         time.Time
	resolveTime     time.Time
	settled         bool
	incomingMsat    lnwire.MilliSatoshi
	outgoingMsat    lnwire.MilliSatoshi
	incomingPeer    route.Vertex
	outgoingPeer    route.Vertex
	incomingCircuit circuitKey
	outgoingCircuit circuitKey
}

// RecordHtlcResolution records a HTLC that has been resolved.
func (d *Db) RecordHtlcResolution(ctx context.Context,
	htlc *HtlcInfo) error {

	if err := d.insertHtlcResolution(ctx, htlc); err != nil {
		return err
	}

	return nil
}

func (d *Db) insertHtlcResolution(ctx context.Context, htlc *HtlcInfo) error {
	insert := `INSERT INTO forwarding_history (
                add_time,
                resolved_time,
                settled,
                incoming_amt_msat,
                outgoing_amt_msat,
                incoming_peer,
                incoming_channel,
                incoming_htlc_index,
                outgoing_peer,
                outgoing_channel,
                outgoing_htlc_index)
                VALUES (?,?,?,?,?,?,?,?,?,?,?);`

	_, err := d.db.ExecContext(
		ctx, insert,
		htlc.addTime.UnixNano(),
		htlc.resolveTime.UnixNano(),
		htlc.settled,
		uint64(htlc.incomingMsat),
		uint64(htlc.outgoingMsat),
		hex.EncodeToString(htlc.incomingPeer[:]),
		htlc.incomingCircuit.channel,
		htlc.incomingCircuit.htlc,
		hex.EncodeToString(htlc.outgoingPeer[:]),
		htlc.outgoingCircuit.channel,
		htlc.outgoingCircuit.htlc,
	)

	return err
}
