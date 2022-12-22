package main

import (
	"context"
	"net"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/lightningequipment/circuitbreaker"
	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func run(c *cli.Context) error {
	// Open database.
	ctx := context.Background()

	db, err := circuitbreaker.NewDb(ctx)
	if err != nil {
		return err
	}

	limit := circuitbreaker.Limit{
		MinIntervalMs: 3,
		MaxPending:    3,
	}
	err = db.SetLimit(ctx, &route.Vertex{1, 2, 4}, limit)
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

	limits, err := db.GetLimits(ctx)
	if err != nil {
		return err
	}

	p := circuitbreaker.NewProcess(client, log, limits)

	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer()),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer()),
	)

	reflection.Register(grpcServer)

	server := circuitbreaker.NewServer(log, p, client, db)

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
