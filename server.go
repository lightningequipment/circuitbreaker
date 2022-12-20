package circuitbreaker

import (
	"context"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
)

type server struct {
	process *process

	circuitbreakerrpc.UnimplementedServiceServer
}

func NewServer(process *process) *server {
	return &server{
		process: process,
	}
}

func (s *server) GetInfo(ctx context.Context,
	req *circuitbreakerrpc.GetInfoRequest) (*circuitbreakerrpc.GetInfoResponse,
	error) {

	return &circuitbreakerrpc.GetInfoResponse{}, nil
}

func (s *server) UpdateLimit(ctx context.Context,
	req *circuitbreakerrpc.UpdateLimitRequest) (
	*circuitbreakerrpc.UpdateLimitResponse, error) {

	var peer *route.Vertex
	if len(req.Node) > 0 {
		node, err := route.NewVertexFromBytes(req.Node)
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

	err := s.process.UpdateLimit(ctx, peer, limit)
	if err != nil {
		return nil, err
	}

	return &circuitbreakerrpc.UpdateLimitResponse{}, nil
}
