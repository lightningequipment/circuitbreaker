package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/lightningnetwork/lnd/routing/route"
	"github.com/urfave/cli"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"gopkg.in/macaroon.v2"
)

type lndclientGrpc struct {
	conn *grpc.ClientConn

	main   lnrpc.LightningClient
	router routerrpc.RouterClient
}

func newLndClient(ctx *cli.Context) (*lndclientGrpc, error) {
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
	macBytes, err := os.ReadFile(macPath)
	if err != nil {
		return nil, fmt.Errorf("unable to read macaroon path (check "+
			"the network setting!): %v", err)
	}

	mac := &macaroon.Macaroon{}
	if err = mac.UnmarshalBinary(macBytes); err != nil {
		return nil, fmt.Errorf("unable to decode macaroon: %v", err)
	}

	// Now we append the macaroon credentials to the dial options.
	cred, err := macaroons.NewMacaroonCredential(mac)
	if err != nil {
		return nil, fmt.Errorf("cannot create mac credential: %w", err)
	}
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

	return &lndclientGrpc{
		conn:   conn,
		main:   lnrpc.NewLightningClient(conn),
		router: routerrpc.NewRouterClient(conn),
	}, nil
}

func (l *lndclientGrpc) getIdentity() (route.Vertex, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

	info, err := l.main.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return route.Vertex{}, err
	}

	return route.NewVertexFromStr(info.IdentityPubkey)
}

type channel struct {
	peer      route.Vertex
	initiator bool
}

func (l *lndclientGrpc) listChannels() (map[uint64]*channel, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

	log.Debugw("Retrieving channels")

	resp, err := l.main.ListChannels(ctx, &lnrpc.ListChannelsRequest{})
	if err != nil {
		return nil, err
	}

	chans := make(map[uint64]*channel)
	for _, rpcChan := range resp.Channels {
		peer, err := route.NewVertexFromStr(rpcChan.RemotePubkey)
		if err != nil {
			return nil, err
		}

		chans[rpcChan.ChanId] = &channel{
			peer:      peer,
			initiator: rpcChan.Initiator,
		}
	}

	return chans, nil
}

func (l *lndclientGrpc) subscribeHtlcEvents(ctx context.Context,
	in *routerrpc.SubscribeHtlcEventsRequest) (
	routerrpc.Router_SubscribeHtlcEventsClient, error) {

	return l.router.SubscribeHtlcEvents(ctx, in)
}

func (l *lndclientGrpc) htlcInterceptor(ctx context.Context) (
	routerrpc.Router_HtlcInterceptorClient, error) {

	return l.router.HtlcInterceptor(ctx)
}

func (l *lndclientGrpc) close() {
	l.conn.Close()
}

func (l *lndclientGrpc) getNodeAlias(key route.Vertex) (string, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

	log.Debugw("Retrieving node info",
		"key", key)

	info, err := l.main.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
		PubKey: key.String(),
	})
	if err != nil {
		return "", err
	}

	if info.Node == nil {
		return "", errors.New("node info not available")
	}

	return info.Node.Alias, nil
}

func (l *lndclientGrpc) getPendingIncomingHtlcs(ctx context.Context) (
	map[circuitKey]struct{}, error) {

	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	resp, err := l.main.ListChannels(ctx, &lnrpc.ListChannelsRequest{})
	if err != nil {
		return nil, err
	}

	htlcs := make(map[circuitKey]struct{})
	for _, channel := range resp.Channels {
		for _, htlc := range channel.PendingHtlcs {
			if !htlc.Incoming {
				continue
			}

			key := circuitKey{
				channel: channel.ChanId,
				htlc:    htlc.HtlcIndex,
			}

			htlcs[key] = struct{}{}
		}
	}

	return htlcs, nil
}
