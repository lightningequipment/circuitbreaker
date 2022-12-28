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

	fmt.Println()

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	headerRow1 := table.Row{"", "LIMITS", "LIMITS", "COUNTERS", "COUNTERS"}
	t.AppendHeader(headerRow1, table.RowConfig{AutoMerge: true})

	headerRow2 := table.Row{"NODE", "MAX HOURLY RATE", "MAX PENDING", "1 HR", "24 HR"}
	t.AppendHeader(headerRow2)

	for _, limit := range resp.Limits {
		var row table.Row
		if limit.Limit == nil {
			row = table.Row{
				limit.Node, "<default>", "<default>",
			}
		} else {
			row = table.Row{
				limit.Node, limit.Limit.MaxHourlyRate, limit.Limit.MaxPending,
			}
		}

		formatCounter := func(counter *circuitbreakerrpc.Counter) string {
			return fmt.Sprintf("%v / %v / %v", counter.Success, counter.Fail, counter.Reject)
		}

		row = append(row,
			formatCounter(limit.Counter_1H),
			formatCounter(limit.Counter_24H),
		)

		t.AppendRow(row)
	}

	fmt.Println("PER NODE LIMITS AND COUNTERS (SUCCESS / FAILED / REJECTED)")
	t.Render()

	return nil
}
