package main

import (
	"context"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/urfave/cli"
)

func updateDefaultLimit(c *cli.Context) error {
	// Open database.
	ctx := context.Background()

	client, err := getClientFromContext(ctx, c)
	if err != nil {
		return err
	}

	req := &circuitbreakerrpc.UpdateDefaultLimitRequest{
		MaxHourlyRate: c.Int64(maxHourlyRateFlag.Name),
		MaxPending:    c.Int64(maxPendingFlag.Name),
	}

	_, err = client.UpdateDefaultLimit(ctx, req)

	return err
}
