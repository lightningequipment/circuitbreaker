package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/lightningnetwork/lnd/build"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	defaultRPCHostPort = "localhost:9234"
)

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
			},
		},
		{
			Name:   "clearlimit",
			Action: clearLimit,
			Flags: []cli.Flag{
				nodeFlag,
			},
		},
		{
			Name:   "updatedefaultlimit",
			Action: updateDefaultLimit,
			Flags: []cli.Flag{
				maxHourlyRateFlag,
				maxPendingFlag,
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
