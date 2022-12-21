package main

import (
	"context"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
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

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"NODE", "MIN_INTERVAL_MS", "BURST_SIZE", "MAX_PENDING"})

	for _, limit := range resp.Limits {
		node := limit.Node
		if node == "" {
			node = "<global>"
		}

		t.AppendRow(table.Row{
			node, limit.MinIntervalMs, limit.BurstSize, limit.MaxPending,
		})
	}

	t.Render()

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
