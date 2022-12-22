package main

import (
	"context"
	"fmt"
	"os"
	"time"

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

	headerRow1 := table.Row{"", "LIMITS", "LIMITS"}
	headerRow2 := table.Row{"NODE", "MAX HOURLY RATE", "MAX PENDING"}
	for _, interval := range resp.CounterIntervalsSec {
		header := fmt.Sprintf("%v", time.Duration(interval)*time.Second)
		headerRow1 = append(headerRow1, "COUNTERS")
		headerRow2 = append(headerRow2, header)
	}
	t.AppendHeader(headerRow1, table.RowConfig{AutoMerge: true})
	t.AppendHeader(headerRow2)

	for _, limit := range resp.Limits {
		var row table.Row
		if limit.Limit == nil {
			row = table.Row{
				limit.Node, "<global>", "<global>",
			}
		} else {
			row = table.Row{
				limit.Node, limit.Limit.MaxHourlyRate, limit.Limit.MaxPending,
			}
		}

		for _, counter := range limit.Counters {
			counterString := fmt.Sprintf("%v / %v", counter.Successes, counter.Total)
			row = append(row, counterString)
		}

		t.AppendRow(row)
	}

	fmt.Println("PER NODE LIMITS AND COUNTERS (SUCCESS / FAILED / REJECTED)")
	t.Render()

	return nil
}

func printGlobalLimit(limit *circuitbreakerrpc.Limit) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	headerRow := table.Row{"MAX HOURLY RATE", "MAX PENDING"}
	t.AppendHeader(headerRow)

	t.AppendRow(table.Row{
		limit.MaxHourlyRate, limit.MaxPending,
	})

	fmt.Println("GLOBAL LIMITS")
	t.Render()
}
