package circuitbreaker

import (
	"context"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
)

type server struct {
	circuitbreakerrpc.UnimplementedServiceServer
}

func NewServer() *server {
	return &server{}
}

func (s *server) GetInfo(ctx context.Context,
	req *circuitbreakerrpc.GetInfoRequest) (*circuitbreakerrpc.GetInfoResponse,
	error) {

	return &circuitbreakerrpc.GetInfoResponse{}, nil
}
