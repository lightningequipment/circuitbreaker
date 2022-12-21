package main

import (
	"context"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/urfave/cli"
)

func updateLimit(c *cli.Context) error {
	// Open database.
	ctx := context.Background()

	client, err := getClientFromContext(ctx, c)
	if err != nil {
		return err
	}

	req := &circuitbreakerrpc.UpdateLimitRequest{
		Node:          c.String("node"),
		MinIntervalMs: c.Int64("min_interval_ms"),
		BurstSize:     c.Int64("burst_size"),
		MaxPending:    c.Int64("max_pending"),
	}

	_, err = client.UpdateLimit(ctx, req)

	return err
}
