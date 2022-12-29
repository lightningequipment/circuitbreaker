package main

import (
	"context"
	"errors"

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

	var mode circuitbreakerrpc.Mode

	modeStr := c.String(modeFlag.Name)
	switch modeStr {
	case modeFail:
		mode = circuitbreakerrpc.Mode_MODE_FAIL

	case modeQueue:
		mode = circuitbreakerrpc.Mode_MODE_QUEUE

	case modeQueuePeerInitiated:
		mode = circuitbreakerrpc.Mode_MODE_QUEUE_PEER_INITIATED

	default:
		return errors.New("unknown mode")
	}

	req := &circuitbreakerrpc.UpdateLimitRequest{
		Node: c.String(nodeFlag.Name),
		Limit: &circuitbreakerrpc.Limit{
			MaxHourlyRate: c.Int64(maxHourlyRateFlag.Name),
			MaxPending:    c.Int64(maxPendingFlag.Name),
			Mode:          mode,
		},
	}

	_, err = client.UpdateLimit(ctx, req)

	return err
}
