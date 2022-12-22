package circuitbreaker

import (
	"context"
	"encoding/hex"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"go.uber.org/zap"
)

type server struct {
	process *process
	lnd     lndclient
	db      *Db
	log     *zap.SugaredLogger

	circuitbreakerrpc.UnimplementedServiceServer
}

func NewServer(log *zap.SugaredLogger, process *process, lnd lndclient,
	db *Db) *server {

	return &server{
		process: process,
		lnd:     lnd,
		db:      db,
		log:     log,
	}
}

func (s *server) GetInfo(ctx context.Context,
	req *circuitbreakerrpc.GetInfoRequest) (*circuitbreakerrpc.GetInfoResponse,
	error) {

	key, err := s.lnd.getIdentity()
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.GetInfoResponse{
		ConnectedNode: hex.EncodeToString(key[:]),
	}, nil
}

func (s *server) UpdateLimit(ctx context.Context,
	req *circuitbreakerrpc.UpdateLimitRequest) (
	*circuitbreakerrpc.UpdateLimitResponse, error) {

	var peer *route.Vertex
	if len(req.Node) > 0 {
		node, err := route.NewVertexFromStr(req.Node)
		if err != nil {
			return nil, err
		}

		peer = &node
	}

	limit := Limit{
		MinIntervalMs: req.MinIntervalMs,
		MaxPending:    req.MaxPending,
	}

	s.log.Infow("Updating limit", "node", peer, "limit", limit)

	err := s.db.SetLimit(ctx, peer, limit)
	if err != nil {
		return nil, err
	}

	err = s.process.UpdateLimit(ctx, peer, limit)
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.UpdateLimitResponse{}, nil
}

func (s *server) ListLimits(ctx context.Context,
	req *circuitbreakerrpc.ListLimitsRequest) (
	*circuitbreakerrpc.ListLimitsResponse, error) {

	limits, err := s.db.GetLimits(ctx)
	if err != nil {
		return nil, err
	}

	counters, err := s.process.getRateCounters(ctx)
	if err != nil {
		return nil, err
	}

	var rpcLimits = []*circuitbreakerrpc.NodeLimit{}

	for peer, limit := range limits.PerPeer {
		rpcLimit := &circuitbreakerrpc.NodeLimit{
			Node: hex.EncodeToString(peer[:]),
			Limit: &circuitbreakerrpc.Limit{
				MinIntervalMs: limit.MinIntervalMs,
				MaxPending:    limit.MaxPending,
			},
		}

		counts, ok := counters[peer]
		if !ok {
			// Report all zeroes.
			counts = make([]rateCounts, len(rateCounterIntervals))
		}

		rpcCounts := make([]*circuitbreakerrpc.Counter, len(counts))

		for idx, count := range counts {
			rpcCounts[idx] = &circuitbreakerrpc.Counter{
				Total:     count.total,
				Successes: count.success,
			}
		}

		rpcLimit.Counters = rpcCounts

		delete(counters, peer)

		rpcLimits = append(rpcLimits, rpcLimit)
	}

	for peer, counts := range counters {
		rpcLimit := &circuitbreakerrpc.NodeLimit{
			Node: hex.EncodeToString(peer[:]),
		}

		rpcCounts := make([]*circuitbreakerrpc.Counter, len(counts))

		for idx, count := range counts {
			rpcCounts[idx] = &circuitbreakerrpc.Counter{
				Total:     count.total,
				Successes: count.success,
			}
		}

		rpcLimit.Counters = rpcCounts

		rpcLimits = append(rpcLimits, rpcLimit)
	}

	intervals := make([]int64, len(rateCounterIntervals))
	for idx, interval := range rateCounterIntervals {
		intervals[idx] = int64(interval.Seconds())
	}

	return &circuitbreakerrpc.ListLimitsResponse{
		GlobalLimit: &circuitbreakerrpc.Limit{
			MinIntervalMs: limits.Global.MinIntervalMs,
			MaxPending:    limits.Global.MaxPending,
		},
		CounterIntervalsSec: intervals,
		Limits:              rpcLimits,
	}, nil
}
