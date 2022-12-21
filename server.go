package circuitbreaker

import (
	"context"
	"encoding/hex"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
)

type server struct {
	process *process
	lnd     lndclient
	db      *Db

	circuitbreakerrpc.UnimplementedServiceServer
}

func NewServer(process *process, lnd lndclient, db *Db) *server {
	return &server{
		process: process,
		lnd:     lnd,
		db:      db,
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
		BurstSize:     req.BurstSize,
		MaxPending:    req.MaxPending,
	}

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

	var rpcLimits = []*circuitbreakerrpc.Limit{
		{
			MinIntervalMs: limits.Global.MinIntervalMs,
			BurstSize:     limits.Global.BurstSize,
			MaxPending:    limits.Global.MaxPending,
		},
	}

	for peer, limit := range limits.PerPeer {
		rpcLimit := &circuitbreakerrpc.Limit{
			Node:          hex.EncodeToString(peer[:]),
			MinIntervalMs: limit.MinIntervalMs,
			BurstSize:     limit.BurstSize,
			MaxPending:    limit.MaxPending,
		}

		rpcLimits = append(rpcLimits, rpcLimit)
	}

	return &circuitbreakerrpc.ListLimitsResponse{
		Limits: rpcLimits,
	}, nil
}
