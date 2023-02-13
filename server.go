package main

import (
	"context"
	"encoding/hex"
	"errors"
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

	info, err := s.lnd.getInfo()
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.GetInfoResponse{
		NodeKey:     hex.EncodeToString(info.nodeKey[:]),
		NodeVersion: info.version,
		NodeAlias:   info.alias,

		Version: BuildVersion,
	}, nil
}

func unmarshalLimit(rpcLimit *circuitbreakerrpc.Limit) (Limit, error) {
	limit := Limit{
		MaxHourlyRate: rpcLimit.MaxHourlyRate,
		MaxPending:    rpcLimit.MaxPending,
	}

	switch rpcLimit.Mode {
	case circuitbreakerrpc.Mode_MODE_FAIL:
		limit.Mode = ModeFail

	case circuitbreakerrpc.Mode_MODE_QUEUE:
		limit.Mode = ModeQueue

	case circuitbreakerrpc.Mode_MODE_QUEUE_PEER_INITIATED:
		limit.Mode = ModeQueuePeerInitiated

	case circuitbreakerrpc.Mode_MODE_BLOCK:
		limit.Mode = ModeBlock

	default:
		return Limit{}, errors.New("unknown mode")
	}

	return limit, nil
}

func (s *server) UpdateLimit(ctx context.Context,
	req *circuitbreakerrpc.UpdateLimitRequest) (
	*circuitbreakerrpc.UpdateLimitResponse, error) {

	node, err := route.NewVertexFromStr(req.Node)
	if err != nil {
		return nil, err
	}

	if node == defaultNodeKey {
		return nil, errors.New("set default limit through UpdateDefaultLimit")
	}

	if req.Limit == nil {
		return nil, errors.New("no limit specified")
	}

	limit, err := unmarshalLimit(req.Limit)
	if err != nil {
		return nil, err
	}

	s.log.Infow("Updating limit", "node", node, "limit", limit)

	err = s.db.UpdateLimit(ctx, node, limit)
	if err != nil {
		return nil, err
	}

	err = s.process.UpdateLimit(ctx, &node, &limit)
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.UpdateLimitResponse{}, nil
}

func (s *server) ClearLimit(ctx context.Context,
	req *circuitbreakerrpc.ClearLimitRequest) (
	*circuitbreakerrpc.ClearLimitResponse, error) {

	node, err := route.NewVertexFromStr(req.Node)
	if err != nil {
		return nil, err
	}

	s.log.Infow("Clearing limit", "node", node)

	err = s.db.ClearLimit(ctx, node)
	if err != nil {
		return nil, err
	}

	err = s.process.UpdateLimit(ctx, &node, nil)
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.ClearLimitResponse{}, nil
}

func (s *server) UpdateDefaultLimit(ctx context.Context,
	req *circuitbreakerrpc.UpdateDefaultLimitRequest) (
	*circuitbreakerrpc.UpdateDefaultLimitResponse, error) {

	limit, err := unmarshalLimit(req.Limit)
	if err != nil {
		return nil, err
	}

	s.log.Infow("Updating default limit", "limit", limit)

	err = s.db.UpdateLimit(ctx, defaultNodeKey, limit)
	if err != nil {
		return nil, err
	}

	err = s.process.UpdateLimit(ctx, nil, &limit)
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.UpdateDefaultLimitResponse{}, nil
}

func marshalLimit(limit Limit) (*circuitbreakerrpc.Limit, error) {
	rpcLimit := &circuitbreakerrpc.Limit{
		MaxHourlyRate: limit.MaxHourlyRate,
		MaxPending:    limit.MaxPending,
	}

	switch limit.Mode {
	case ModeFail:
		rpcLimit.Mode = circuitbreakerrpc.Mode_MODE_FAIL

	case ModeQueue:
		rpcLimit.Mode = circuitbreakerrpc.Mode_MODE_QUEUE

	case ModeQueuePeerInitiated:
		rpcLimit.Mode = circuitbreakerrpc.Mode_MODE_QUEUE_PEER_INITIATED

	case ModeBlock:
		rpcLimit.Mode = circuitbreakerrpc.Mode_MODE_BLOCK

	default:
		return nil, errors.New("unknown mode")
	}

	return rpcLimit, nil
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

	createRpcState := func(peer route.Vertex, state *peerState) (
		*circuitbreakerrpc.NodeLimit, error) {

		alias, err := s.getAlias(peer)
		if err != nil {
			return nil, err
		}

		return &circuitbreakerrpc.NodeLimit{
			Node:             hex.EncodeToString(peer[:]),
			Alias:            alias,
			Counter_1H:       marshalCounter(state.counts[0]),
			Counter_24H:      marshalCounter(state.counts[1]),
			QueueLen:         state.queueLen,
			PendingHtlcCount: state.pendingHtlcCount,
		}, nil
	}

	for peer, limit := range limits.PerPeer {
		counts, ok := counters[peer]
		if !ok {
			// Report all zeroes.
			counts = &peerState{
				counts: make([]rateCounts, len(rateCounterIntervals)),
			}
		}

		rpcState, err := createRpcState(peer, counts)
		if err != nil {
			return nil, err
		}

		rpcLimit, err := marshalLimit(limit)
		if err != nil {
			return nil, err
		}
		rpcState.Limit = rpcLimit

		delete(counters, peer)

		rpcLimits = append(rpcLimits, rpcState)
	}

	for peer, counts := range counters {
		rpcLimit, err := createRpcState(peer, counts)
		if err != nil {
			return nil, err
		}

		rpcLimits = append(rpcLimits, rpcLimit)
	}

	defaultLimit, err := marshalLimit(limits.Default)
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.ListLimitsResponse{
		DefaultLimit: defaultLimit,
		Limits:       rpcLimits,
	}, nil
}
