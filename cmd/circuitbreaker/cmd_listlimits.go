package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"

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

	headerRow1 := table.Row{"", "", "LIMITS", "LIMITS", "LIMITS", "COUNTERS", "COUNTERS"}
	t.AppendHeader(headerRow1, table.RowConfig{AutoMerge: true})

	headerRow2 := table.Row{"NODE", "ALIAS", "MAX HOURLY RATE", "MAX PENDING", "MODE", "1 HR", "24 HR"}
	t.AppendHeader(headerRow2)

	for _, limit := range resp.Limits {
		row := table.Row{
			limit.Node, limit.Alias,
		}

		appendNumber := func(number int64) {
			val := "-"
			if number != 0 {
				val = strconv.FormatInt(number, 10)
			}

			row = append(row, val)
		}

		appendNumber(limit.Limit.MaxHourlyRate)
		appendNumber(limit.Limit.MaxPending)

		var modeStr string
		switch limit.Limit.Mode {
		case circuitbreakerrpc.Mode_MODE_FAIL:
			modeStr = modeFail

		case circuitbreakerrpc.Mode_MODE_QUEUE:
			modeStr = modeQueue

		case circuitbreakerrpc.Mode_MODE_QUEUE_PEER_INITIATED:
			modeStr = modeQueuePeerInitiated

		default:
			return errors.New("unknown mode")
		}

		row = append(row, modeStr)

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
