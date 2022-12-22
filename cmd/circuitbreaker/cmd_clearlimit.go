package main

import (
	"context"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/urfave/cli"
)

func clearLimit(c *cli.Context) error {
	ctx := context.Background()

	client, err := getClientFromContext(ctx, c)
	if err != nil {
		return err
	}

	req := &circuitbreakerrpc.ClearLimitRequest{
		Node: c.String(nodeFlag.Name),
	}

	_, err = client.ClearLimit(ctx, req)

	return err
}
