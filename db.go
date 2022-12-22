package circuitbreaker

import (
	"context"
	"database/sql"
	"errors"

	"github.com/lightningnetwork/lnd/routing/route"
	_ "modernc.org/sqlite"
)

var defaultNodeKey = route.Vertex{}

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
		node_in BLOB PRIMARY KEY NOT NULL,
		htlc_max_pending INTEGER NOT NULL,
		htlc_max_hourly_rate INTEGER NOT NULL
	);
	
	INSERT OR IGNORE INTO limits(node_in, htlc_max_pending, htlc_max_hourly_rate) VALUES(?, 2, 360);
	`

	if _, err := db.ExecContext(ctx, initQuery, defaultNodeKey[:]); err != nil {
		return nil, err
	}

	return &Db{
		db: db,
	}, nil
}

type Limit struct {
	MaxHourlyRate int64
	MaxPending    int64
}

type Limits struct {
	Default Limit
	PerPeer map[route.Vertex]Limit
}

func (d *Db) UpdateLimit(ctx context.Context, peer route.Vertex,
	limit Limit) error {

	const replace string = `REPLACE INTO limits(node_in, htlc_max_pending, htlc_max_hourly_rate) VALUES(?, ?, ?);`

	_, err := d.db.ExecContext(
		ctx, replace, peer[:],
		limit.MaxPending, limit.MaxHourlyRate,
	)

	return err
}

func (d *Db) UpdateDefaultLimit(ctx context.Context, limit Limit) error {
	const replace string = `REPLACE INTO limits(node_in, htlc_max_pending, htlc_max_hourly_rate) VALUES(?, ?, ?);`

	_, err := d.db.ExecContext(
		ctx, replace, defaultNodeKey[:],
		limit.MaxPending, limit.MaxHourlyRate,
	)

	return err
}

func (d *Db) ClearLimit(ctx context.Context, peer route.Vertex) error {
	if peer == defaultNodeKey {
		return errors.New("cannot clear default limits")
	}

	const query string = `DELETE FROM limits WHERE node_in = ?;`

	_, err := d.db.ExecContext(
		ctx, query, peer[:],
	)

	return err
}

func (d *Db) GetLimits(ctx context.Context) (*Limits, error) {
	const query string = `
	SELECT node_in, htlc_max_pending, htlc_max_hourly_rate from limits;`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}

	var limits = Limits{
		PerPeer: make(map[route.Vertex]Limit),
	}
	for rows.Next() {
		var (
			limit  Limit
			nodeIn []byte
		)
		err := rows.Scan(
			&nodeIn, &limit.MaxPending, &limit.MaxHourlyRate,
		)
		if err != nil {
			return nil, err
		}

		key, err := route.NewVertexFromBytes(nodeIn)
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
