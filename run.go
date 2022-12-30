package main

import (
	"context"
	"net"
	"net/http"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

func run(c *cli.Context) error {
	ctx := context.Background()

	// Open database.
	db, err := NewDb(ctx)
	if err != nil {
		return err
	}

	// First, we'll parse the args from the command.
	tlsCertPath, macPath, err := extractPathArgs(c)
	if err != nil {
		return err
	}

	lndCfg := LndConfig{
		RpcServer:   c.GlobalString("rpcserver"),
		TlsCertPath: tlsCertPath,
		MacPath:     macPath,
		Log:         log,
	}

	client, err := NewLndClient(&lndCfg)
	if err != nil {
		return err
	}
	defer client.Close()

	limits, err := db.GetLimits(ctx)
	if err != nil {
		return err
	}

	p := NewProcess(client, log, limits)

	grpcServer := grpc.NewServer(
		grpc.StreamInterceptor(grpc_middleware.ChainStreamServer()),
		grpc.UnaryInterceptor(grpc_middleware.ChainUnaryServer()),
	)

	reflection.Register(grpcServer)

	server := NewServer(log, p, client, db)

	circuitbreakerrpc.RegisterServiceServer(
		grpcServer, server,
	)

	listenAddress := c.String("listen")
	grpcInternalListener, err := net.Listen("tcp", listenAddress)
	if err != nil {
		return err
	}

	group, ctx := errgroup.WithContext(ctx)

	group.Go(func() error {
		// Create a client connection to the gRPC server we just started
		// This is where the gRPC-Gateway proxies the requests
		conn, err := grpc.DialContext(
			ctx,
			listenAddress,
			grpc.WithBlock(),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return err
		}

		gwmux := runtime.NewServeMux()

		// Register Greeter
		err = circuitbreakerrpc.RegisterServiceHandler(ctx, gwmux, conn)
		if err != nil {
			return err
		}

		fs := http.FileServer(http.Dir("webui/build/"))
		mux := http.NewServeMux()
		mux.Handle("/api/", http.StripPrefix("/api", gwmux))
		mux.HandleFunc("/", fs.ServeHTTP)

		restListen := c.String(restListenFlag.Name)
		gwServer := &http.Server{
			Addr:    restListen,
			Handler: mux,
		}

		log.Infow("HTTP server starting", "listenAddress", restListen)

		return gwServer.ListenAndServe()
	})

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
