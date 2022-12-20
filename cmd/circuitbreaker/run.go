package main

import (
	"context"
	"net"
	"path/filepath"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/lightningequipment/circuitbreaker"
	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer()),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer()),
	)

	reflection.Register(grpcServer)

	server := circuitbreaker.NewServer()

	circuitbreakerrpc.RegisterServiceServer(
		grpcServer, server,
	)

	listenAddress := c.String("listen")
	grpcInternalListener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(context.Background())

	// Run circuitbreaker core.
	group.Go(func() error {
		return p.Run(ctx)
	})

	// Run grpc server.
	group.Go(func() error {
		log.Infow("Grpc server starting", "listenAddress", listenAddress)
		err := grpcServer.Serve(grpcInternalListener)
		if err != nil && err != grpc.ErrServerStopped {
			log.Errorw("grpc server error", "err", err)
		}

		return err
	})

	group.Go(func() error {
		<-ctx.Done()

		// Stop grpc server.
		log.Infof("Stopping grpc server")
		grpcServer.Stop()

		return nil
	})

	return group.Wait()
}
