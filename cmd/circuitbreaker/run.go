package main

import (
	"context"
	"path/filepath"

	"github.com/lightningequipment/circuitbreaker"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
)

func run(c *cli.Context) error {
	configPath := filepath.Join(c.String("configdir"), confFn)
	loader := newConfigLoader(configPath)

	config, err := loader.load()
	if err != nil {
		return err
	}

	// First, we'll parse the args from the command.
	tlsCertPath, macPath, err := extractPathArgs(c)
	if err != nil {
		return err
	}

	lndCfg := circuitbreaker.LndConfig{
		RpcServer:   c.GlobalString("rpcserver"),
		TlsCertPath: tlsCertPath,
		MacPath:     macPath,
		Log:         log,
	}

	client, err := circuitbreaker.NewLndClient(&lndCfg)
	if err != nil {
		return err
	}
	defer client.Close()

	p := circuitbreaker.NewProcess(client, log, config)

	group, ctx := errgroup.WithContext(context.Background())

	// Run circuitbreaker core.
	group.Go(func() error {
		return p.Run(ctx)
	})

	return group.Wait()
}
