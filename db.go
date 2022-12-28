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

	return &Db{
		db: db,
	}, nil
}

type Limit struct {
	MaxHourlyRate int64
	MaxPending    int64
}

type Limits struct {
	PerPeer map[route.Vertex]Limit
}

func (d *Db) UpdateLimit(ctx context.Context, peer route.Vertex,
	limit Limit) error {

	if limit.MaxHourlyRate == 0 && limit.MaxPending == 0 {
		const delete string = `DELETE FROM limits WHERE node_in=?;`

		_, err := d.db.ExecContext(
			ctx, delete, peer[:],
		)

		return err
	}

	const replace string = `REPLACE INTO limits(node_in, htlc_max_pending, htlc_max_hourly_rate) VALUES(?, ?, ?);`

	_, err := d.db.ExecContext(
		ctx, replace, peer[:],
		limit.MaxPending, limit.MaxHourlyRate,
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

		limits.PerPeer[key] = limit
	}

	return &limits, nil
}
