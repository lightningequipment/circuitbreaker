package main

import (
	"context"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/urfave/cli"
)

func listLimits(c *cli.Context) error {
	// Open database.
	ctx := context.Background()

	client, err := getClientFromContext(ctx, c)
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
