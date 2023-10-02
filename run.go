package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/lightningequipment/circuitbreaker/circuitbreakerrpc"
	"github.com/urfave/cli"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

var errUserExit = errors.New("user requested termination")

//go:embed all:webui-build
var content embed.FS

func run(c *cli.Context) error {
	ctx := context.Background()

	confDir := c.String("configdir")
	err := os.MkdirAll(confDir, os.ModePerm)
	if err != nil {
		return err
	}
	dbPath := filepath.Join(confDir, dbFn)

	log.Infow("Circuit Breaker starting", "version", BuildVersion)

	log.Infow("Opening database", "path", dbPath)

	// Open database.
	db, err := NewDb(dbPath, c.Int("fwdhistorylimit"))
	if err != nil {
		return err
	}
	defer func() {
		err := db.Close()
		if err != nil {
			log.Errorw("Error closing db", "err", err)
		}
	}()

	group, ctx := errgroup.WithContext(ctx)

	stub := c.Bool(stubFlag.Name)
	var client lndclient
	if stub {
		stubClient := newStubClient(ctx)

		client = stubClient
	} else {
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

		lndClient, err := NewLndClient(&lndCfg)
		if err != nil {
			return err
		}
		defer lndClient.Close()

		client = lndClient
	}

	limits, err := db.GetLimits(ctx)
	if err != nil {
		return err
	}

	p := NewProcess(client, log, limits, db)

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

	// Create a client connection to the gRPC server we just started
	// This is where the gRPC-Gateway proxies the requests
	conn, err := grpc.DialContext(
		ctx,
		listenAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}

	// Create http server.
	gwmux := runtime.NewServeMux()

	err = circuitbreakerrpc.RegisterServiceHandler(ctx, gwmux, conn)
	if err != nil {
		return err
	}

	serverRoot, err := fs.Sub(content, "webui-build")
	if err != nil {
		log.Fatal(err)
	}

	fs := http.FileServer(http.FS(serverRoot))
	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", gwmux))
	mux.HandleFunc("/", fs.ServeHTTP)

	httpListen := c.String(httpListenFlag.Name)
	gwServer := &http.Server{
		Addr:              httpListen,
		Handler:           mux,
		ReadHeaderTimeout: time.Second * 10,
	}

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

	// Run http server.
	group.Go(func() error {
		log.Infow("HTTP server starting", "listenAddress", httpListen)

		return gwServer.ListenAndServe()
	})

	// Stop servers when context is cancelled.
	group.Go(func() error {
		<-ctx.Done()

		// Stop http server.
		log.Infof("Stopping http server")
		err := gwServer.Shutdown(context.Background()) //nolint:contextcheck
		if err != nil {
			log.Errorw("Error shutting down http server", "err", err)
		}

		// Stop grpc server.
		log.Infof("Stopping grpc server")
		grpcServer.Stop()

		return nil
	})

	group.Go(func() error {
		log.Infof("Press ctrl-c to exit")

		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

		select {
		case <-sigint:
			return errUserExit

		case <-ctx.Done():
			return nil
		}
	})

	return group.Wait()
}
