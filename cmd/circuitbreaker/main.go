package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/lightningnetwork/lnd/build"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	defaultRPCHostPort = "localhost:9234"
)

type EnumValue struct {
	Enum     []string
	Default  string
	selected string
}

func (e *EnumValue) Set(value string) error {
	for _, enum := range e.Enum {
		if enum == value {
			e.selected = value
			return nil
		}
	}

	return fmt.Errorf("allowed values are %s", strings.Join(e.Enum, ", "))
}

func (e EnumValue) String() string {
	if e.selected == "" {
		return e.Default
	}
	return e.selected
}

var (
	nodeFlag = cli.StringFlag{
		Name:  "node",
		Usage: "node pubkey",
	}

	maxHourlyRateFlag = cli.Int64Flag{
		Name:     "max_hourly_rate",
		Required: true,
	}

	maxPendingFlag = cli.Int64Flag{
		Name:     "max_pending",
		Required: true,
	}

	modeFail               = "fail"
	modeQueue              = "queue"
	modeQueuePeerInitiated = "queue_peer_initiated"

	modeFlag = cli.GenericFlag{
		Name: "mode",
		Value: &EnumValue{
			Enum:    []string{modeFail, modeQueue, modeQueuePeerInitiated},
			Default: modeFail,
		},
		Usage: strings.Join([]string{modeFail, modeQueue, modeQueuePeerInitiated}, " / "),
	}
)

func main() {
	app := cli.NewApp()
	app.Name = "circuitbreaker"
	app.Version = build.Version() + " commit=" + build.Commit
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "rpcserver",
			Value: defaultRPCHostPort,
			Usage: "host:port of circuitbreaker daemon",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:   "listlimits",
			Action: listLimits,
		},
		{
			Name:   "updatelimit",
			Action: updateLimit,
			Flags: []cli.Flag{
				nodeFlag,
				maxHourlyRateFlag,
				maxPendingFlag,
				modeFlag,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("Unexpected exit: %v\n", err)
	}
}

func getClientFromContext(ctx context.Context, c *cli.Context) (
	circuitbreakerrpc.ServiceClient, error) {

	return getClient(ctx, c.GlobalString("rpcserver"))
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
