package circuitbreaker

import (
	"context"
	"database/sql"

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

	const create string = `
	CREATE TABLE IF NOT EXISTS limits (
		node_in BLOB PRIMARY KEY,
		htlc_min_interval INTEGER NOT NULL,
		htlc_max_hourly_rate INTEGER NOT NULL
	);`

	if _, err := db.ExecContext(ctx, create); err != nil {
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
	Global  Limit
	PerPeer map[route.Vertex]Limit
}

func (d *Db) SetLimit(ctx context.Context, peer *route.Vertex,
	limit Limit) error {

	const replace string = `
	REPLACE INTO limits(node_in, htlc_min_interval, htlc_max_hourly_rate) VALUES(?, ?, ?);`

	var peerSlice []byte
	if peer != nil {
		peerSlice = peer[:]
	}

	_, err := d.db.ExecContext(
		ctx, replace, peerSlice,
		limit.MaxHourlyRate, limit.MaxPending,
	)
	if err != nil {
		return err
	}

	return nil
}

func (d *Db) GetLimits(ctx context.Context) (*Limits, error) {
	const query string = `
	SELECT node_in, htlc_min_interval, htlc_max_hourly_rate from limits;`

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
			&nodeIn, &limit.MaxHourlyRate, &limit.MaxPending,
		)
		if err != nil {
			return nil, err
		}

		if nodeIn == nil {
			limits.Global = limit
		} else {
			key, err := route.NewVertexFromBytes(nodeIn)
			if err != nil {
				return nil, err
			}

			limits.PerPeer[key] = limit
		}
	}

	return &limits, nil
}
