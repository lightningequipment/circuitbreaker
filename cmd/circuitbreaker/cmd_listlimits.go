package main

import (
	"context"
	"fmt"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func listLimits(c *cli.Context) error {
	// Open database.
	ctx := context.Background()

	client, err := getClient(ctx, c.GlobalString("rpcserver"))
	if err != nil {
		return err
	}

	resp, err := client.ListLimits(ctx, &circuitbreakerrpc.ListLimitsRequest{})
	if err != nil {
		return err
	}

	for _, limit := range resp.Limits {
		fmt.Printf("%v\n", limit.Node)
	}

	return nil
}

func getClient(ctx context.Context, host string) (
	circuitbreakerrpc.ServiceClient, error) {

	insecure := insecure.NewCredentials()

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure),
	}

	conn, err := grpc.DialContext(ctx, host, opts...)
	if err != nil {
		return nil, err
	}

	return circuitbreakerrpc.NewServiceClient(conn), nil
}
