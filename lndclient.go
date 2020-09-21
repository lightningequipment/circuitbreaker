package main

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/lightningnetwork/lnd/routing/route"
	"go.uber.org/zap"

	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"

	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

type lndclient struct {
	conn *grpc.ClientConn

	main   lnrpc.LightningClient
	router routerrpc.RouterClient
}

func newLndClient(ctx *cli.Context) (*lndclient, error) {
	// First, we'll parse the args from the command.
	tlsCertPath, macPath, err := extractPathArgs(ctx)
	if err != nil {
		return nil, err
	}

	// Load the specified TLS certificate and build transport credentials
	// with it.
	creds, err := credentials.NewClientTLSFromFile(tlsCertPath, "")
	if err != nil {
		return nil, err
	}

	// Create a dial options array.
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
	}

	// Load the specified macaroon file.
	macBytes, err := ioutil.ReadFile(macPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read macaroon path (check "+
			"the network setting!): %v", err)
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macBytes); err != nil {
		return nil, fmt.Errorf("unable to decode macaroon: %v", err)
	}

	// Now we append the macaroon credentials to the dial options.
	cred := macaroons.NewMacaroonCredential(mac)
	opts = append(opts, grpc.WithPerRPCCredentials(cred))

	// We need to use a custom dialer so we can also connect to unix sockets
	// and not just TCP addresses.
	genericDialer := lncfg.ClientAddressDialer(defaultRPCPort)
	opts = append(opts, grpc.WithContextDialer(genericDialer))
	opts = append(opts, grpc.WithDefaultCallOptions(maxMsgRecvSize))

	conn, err := grpc.Dial(ctx.GlobalString("rpcserver"), opts...)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to connect to RPC server: %v", err)
	}

	return &lndclient{
		conn:   conn,
		main:   lnrpc.NewLightningClient(conn),
		router: routerrpc.NewRouterClient(conn),
	}, nil
}

func (l *lndclient) getIdentity() (route.Vertex, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

	info, err := l.main.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return route.Vertex{}, err
	}

	return route.NewVertexFromStr(info.IdentityPubkey)
}

type channelEdge struct {
	node1Pub, node2Pub route.Vertex
}

func (l *lndclient) getChanInfo(channel uint64) (*channelEdge, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

	log.Debug("Retrieving channel info", zap.Uint64("channel", channel))

	info, err := l.main.GetChanInfo(ctx, &lnrpc.ChanInfoRequest{
		ChanId: channel,
	})
	if err != nil {
		return nil, err
	}

	node1Pub, err := route.NewVertexFromStr(info.Node1Pub)
	if err != nil {
		return nil, err
	}

	node2Pub, err := route.NewVertexFromStr(info.Node2Pub)
	if err != nil {
		return nil, err
	}

	return &channelEdge{
		node1Pub: node1Pub,
		node2Pub: node2Pub,
	}, nil
}

func (l *lndclient) close() {
	l.conn.Close()
}
