package circuitbreaker

import (
	"context"
	"encoding/hex"
	"sync"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"go.uber.org/zap"
)

type server struct {
	process *process
	lnd     lndclient
	db      *Db
	log     *zap.SugaredLogger

	aliases     map[route.Vertex]string
	aliasesLock sync.Mutex

	circuitbreakerrpc.UnimplementedServiceServer
}

func NewServer(log *zap.SugaredLogger, process *process,
	lnd lndclient, db *Db) *server {

	return &server{
		process: process,
		lnd:     lnd,
		db:      db,
		log:     log,
		aliases: make(map[route.Vertex]string),
	}
}

func (s *server) getAlias(key route.Vertex) (string, error) {
	s.aliasesLock.Lock()
	defer s.aliasesLock.Unlock()

	alias, ok := s.aliases[key]
	if ok {
		return alias, nil
	}

	alias, err := s.lnd.getNodeAlias(key)
	switch {
	case err == ErrNodeNotFound:

	case err != nil:
		return "", err
	}

	s.aliases[key] = alias

	return alias, nil
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

	err = s.db.UpdateLimit(ctx, node, limit)
	if err != nil {
		return nil, err
	}

	err = s.process.UpdateLimit(ctx, node, limit)
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

	marshalCounter := func(count rateCounts) *circuitbreakerrpc.Counter {
		return &circuitbreakerrpc.Counter{
			Success: count.success,
			Fail:    count.fail,
			Reject:  count.reject,
		}
	}

	var rpcLimits = []*circuitbreakerrpc.NodeLimit{}

	createRpcLimit := func(peer route.Vertex, counts []rateCounts) (
		*circuitbreakerrpc.NodeLimit, error) {

		alias, err := s.getAlias(peer)
		if err != nil {
			return nil, err
		}

		return &circuitbreakerrpc.NodeLimit{
			Node:        hex.EncodeToString(peer[:]),
			Alias:       alias,
			Counter_1H:  marshalCounter(counts[0]),
			Counter_24H: marshalCounter(counts[1]),
			Limit:       &circuitbreakerrpc.Limit{},
		}, nil
	}

	for peer, limit := range limits.PerPeer {
		counts, ok := counters[peer]
		if !ok {
			// Report all zeroes.
			counts = make([]rateCounts, len(rateCounterIntervals))
		}

		rpcLimit, err := createRpcLimit(peer, counts)
		if err != nil {
			return nil, err
		}

		rpcLimit.Limit.MaxHourlyRate = limit.MaxHourlyRate
		rpcLimit.Limit.MaxPending = limit.MaxPending

		delete(counters, peer)

		rpcLimits = append(rpcLimits, rpcLimit)
	}

	for peer, counts := range counters {
		rpcLimit, err := createRpcLimit(peer, counts)
		if err != nil {
			return nil, err
		}

		rpcLimits = append(rpcLimits, rpcLimit)
	}

	return &circuitbreakerrpc.ListLimitsResponse{
		Limits: rpcLimits,
	}, nil
}
