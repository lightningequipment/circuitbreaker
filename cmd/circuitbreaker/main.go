package main

import (
	"fmt"
	"os"

	"github.com/lightningnetwork/lnd/build"
	"github.com/urfave/cli"
)

var (
	defaultRPCHostPort = "localhost:9234"
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
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("Unexpected exit: %v\n", err)
	}
}
