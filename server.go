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

	node, err := route.NewVertexFromStr(req.Node)
	if err != nil {
		return nil, err
	}

	limit := Limit{
		MaxHourlyRate: req.MaxHourlyRate,
		MaxPending:    req.MaxPending,
	}

	s.log.Infow("Updating limit", "node", node, "limit", limit)

	err = s.db.SetLimit(ctx, &node, limit)
	if err != nil {
		return nil, err
	}

	err = s.process.UpdateLimit(ctx, &node, limit)
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.UpdateLimitResponse{}, nil
}

func (s *server) UpdateDefaultLimit(ctx context.Context,
	req *circuitbreakerrpc.UpdateDefaultLimitRequest) (
	*circuitbreakerrpc.UpdateDefaultLimitResponse, error) {

	limit := Limit{
		MaxHourlyRate: req.MaxHourlyRate,
		MaxPending:    req.MaxPending,
	}

	s.log.Infow("Updating default limit", "limit", limit)

	err := s.db.SetLimit(ctx, nil, limit)
	if err != nil {
		return nil, err
	}

	err = s.process.UpdateLimit(ctx, nil, limit)
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.UpdateDefaultLimitResponse{}, nil
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
				MaxHourlyRate: limit.MaxHourlyRate,
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
				Success: count.success,
				Fail:    count.fail,
				Reject:  count.reject,
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
				Success: count.success,
				Fail:    count.fail,
				Reject:  count.reject,
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
			MaxHourlyRate: limits.Global.MaxHourlyRate,
			MaxPending:    limits.Global.MaxPending,
		},
		CounterIntervalsSec: intervals,
		Limits:              rpcLimits,
	}, nil
}
