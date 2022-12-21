package main

import (
	"context"
	"fmt"
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

	printGlobalLimit(resp.GlobalLimit)

	fmt.Println()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	headerRow1 := table.Row{"NODE", "RATE LIMIT", "RATE LIMIT", "MAX PENDING"}
	headerRow2 := table.Row{"", "MIN INTERVAL MS", "BURST SIZE", ""}
	for _, interval := range resp.CounterIntervalsSec {
		header := fmt.Sprintf("%v SEC", interval)
		headerRow1 = append(headerRow1, header, header)

		headerRow2 = append(headerRow2, "SUCCESS", "TOTAL")
	}

	t.AppendHeader(headerRow1, table.RowConfig{AutoMerge: true})
	t.AppendHeader(headerRow2)

	for _, limit := range resp.Limits {
		var row table.Row
		if limit.Limit == nil {
			row = table.Row{
				limit.Node, "<global>", "<global>", "<global>",
			}
		} else {
			row = table.Row{
				limit.Node, limit.Limit.MinIntervalMs, limit.Limit.BurstSize, limit.Limit.MaxPending,
			}
		}

		for _, counter := range limit.Counters {
			row = append(row, counter.Successes, counter.Total)
		}

		t.AppendRow(row)
	}

	t.Render()

	return nil
}

func printGlobalLimit(limit *circuitbreakerrpc.Limit) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	headerRow1 := table.Row{"RATE LIMIT", "RATE LIMIT", "MAX PENDING"}
	headerRow2 := table.Row{"MIN INTERVAL MS", "BURST SIZE", ""}
	t.AppendHeader(headerRow1, table.RowConfig{AutoMerge: true})
	t.AppendHeader(headerRow2)

	t.AppendRow(table.Row{
		limit.MinIntervalMs, limit.BurstSize, limit.MaxPending,
	})

	t.Render()
}
