package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnrpc/routerrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/macaroons"
	"github.com/lightningnetwork/lnd/routing/route"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"gopkg.in/macaroon.v2"
)

var (
	// maxMsgRecvSize is the largest message our client will receive. We
	// set this to 200MiB atm.
	maxMsgRecvSize = grpc.MaxCallRecvMsgSize(1 * 1024 * 1024 * 200)

	ErrNodeNotFound = errors.New("node info not available")

	ctxb = context.Background()
)

type lndclientGrpc struct {
	conn *grpc.ClientConn
	log  *zap.SugaredLogger

	main   lnrpc.LightningClient
	router routerrpc.RouterClient
}

type htlcEventsClient interface {
	recv() (*resolvedEvent, error)
}

type lndHtlcEventsClient struct {
	client routerrpc.Router_SubscribeHtlcEventsClient
}

func (h *lndHtlcEventsClient) recvInternal() (*resolvedEvent, error) {
	event, err := h.client.Recv()
	if err != nil {
		return nil, err
	}

	if event.EventType != routerrpc.HtlcEvent_FORWARD {
		return nil, nil
	}

	var settled bool
	switch event.Event.(type) {
	case *routerrpc.HtlcEvent_SettleEvent:
		settled = true

	case *routerrpc.HtlcEvent_ForwardFailEvent:
	case *routerrpc.HtlcEvent_LinkFailEvent:

	default:
		return nil, nil
	}

	return &resolvedEvent{
		settled: settled,
		incomingCircuitKey: circuitKey{
			channel: event.IncomingChannelId,
			htlc:    event.IncomingHtlcId,
		},
		outgoingCircuitKey: circuitKey{
			channel: event.OutgoingChannelId,
			htlc:    event.OutgoingHtlcId,
		},
		timestamp: time.Unix(0, int64(event.TimestampNs)),
	}, nil
}

func (h *lndHtlcEventsClient) recv() (*resolvedEvent, error) {
	for {
		event, err := h.recvInternal()
		if err != nil {
			return nil, err
		}

		if event != nil {
			return event, nil
		}
	}
}

type htlcInterceptorClient interface {
	recv() (*interceptedEvent, error)
	send(*interceptResponse) error
}

type lndHtlcInterceptorClient struct {
	client routerrpc.Router_HtlcInterceptorClient
}

type interceptedEvent struct {
	circuitKey   circuitKey
	incomingMsat lnwire.MilliSatoshi
	outgoingMsat lnwire.MilliSatoshi
}

func (h *lndHtlcInterceptorClient) recv() (*interceptedEvent, error) {
	event, err := h.client.Recv()
	if err != nil {
		return nil, err
	}

	return &interceptedEvent{
		circuitKey: circuitKey{
			channel: event.IncomingCircuitKey.ChanId,
			htlc:    event.IncomingCircuitKey.HtlcId,
		},
		incomingMsat: lnwire.MilliSatoshi(event.IncomingAmountMsat),
		outgoingMsat: lnwire.MilliSatoshi(event.OutgoingAmountMsat),
	}, nil
}

type interceptResponse struct {
	key    circuitKey
	resume bool
}

func (h *lndHtlcInterceptorClient) send(resp *interceptResponse) error {
	response := &routerrpc.ForwardHtlcInterceptResponse{
		IncomingCircuitKey: &routerrpc.CircuitKey{
			ChanId: resp.key.channel,
			HtlcId: resp.key.htlc,
		},
	}
	if resp.resume {
		response.Action = routerrpc.ResolveHoldForwardAction_RESUME
	} else {
		response.Action = routerrpc.ResolveHoldForwardAction_FAIL
	}

	return h.client.Send(response)
}

type LndConfig struct {
	TlsCertPath, MacPath, RpcServer string
	Log                             *zap.SugaredLogger
}

func NewLndClient(cfg *LndConfig) (*lndclientGrpc, error) {
	// Load the specified TLS certificate and build transport credentials
	// with it.
	creds, err := credentials.NewClientTLSFromFile(cfg.TlsCertPath, "")
	if err != nil {
		return nil, err
	}

	// Load the specified macaroon file.
	macBytes, err := os.ReadFile(cfg.MacPath)
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

	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(maxMsgRecvSize),
		grpc.WithPerRPCCredentials(cred),
	}

	conn, err := grpc.Dial(cfg.RpcServer, opts...)
	if err != nil {
		return nil, fmt.Errorf(
			"unable to connect to RPC server: %v", err)
	}

	return &lndclientGrpc{
		log:    cfg.Log,
		conn:   conn,
		main:   lnrpc.NewLightningClient(conn),
		router: routerrpc.NewRouterClient(conn),
	}, nil
}

type info struct {
	nodeKey route.Vertex
	alias   string
	version string
}

func (l *lndclientGrpc) getInfo() (*info, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

	infoResp, err := l.main.GetInfo(ctx, &lnrpc.GetInfoRequest{})
	if err != nil {
		return nil, err
	}

	nodeKey, err := route.NewVertexFromStr(infoResp.IdentityPubkey)
	if err != nil {
		return nil, err
	}

	return &info{
		nodeKey: nodeKey,
		alias:   infoResp.Alias,
		version: infoResp.Version,
	}, nil
}

type channel struct {
	peer      route.Vertex
	initiator bool
}

func (l *lndclientGrpc) listChannels() (map[uint64]*channel, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

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

func (l *lndclientGrpc) listClosedChannels() (map[uint64]*channel, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

	resp, err := l.main.ClosedChannels(ctx, &lnrpc.ClosedChannelsRequest{})
	if err != nil {
		return nil, err
	}

	chans := make(map[uint64]*channel)
	for _, rpcChan := range resp.Channels {
		peer, err := route.NewVertexFromStr(rpcChan.RemotePubkey)
		if err != nil {
			return nil, err
		}

		channel := &channel{
			peer: peer,
		}

		// LND didn't always store who initiated the channel, so in some cases
		// we don't know who initiated the channel (for very old channels). We're
		// unlikely to hit this case since we're dealing with channels related
		// to current forwards, so we just log that we don't know this value and
		// allow initiator to be true.
		switch rpcChan.OpenInitiator {
		case lnrpc.Initiator_INITIATOR_LOCAL:
			channel.initiator = true

		case lnrpc.Initiator_INITIATOR_REMOTE:

		default:
			channel.initiator = true
			log.Debugf("Channel initiator for %v with %v unknown",
				rpcChan.ChanId, peer)
		}

		chans[rpcChan.ChanId] = channel
	}

	return chans, nil
}

func (l *lndclientGrpc) subscribeHtlcEvents(ctx context.Context) (
	htlcEventsClient, error) {

	req := &routerrpc.SubscribeHtlcEventsRequest{}

	client, err := l.router.SubscribeHtlcEvents(ctx, req)
	if err != nil {
		return nil, err
	}

	return &lndHtlcEventsClient{client: client}, nil
}

func (l *lndclientGrpc) htlcInterceptor(ctx context.Context) (
	htlcInterceptorClient, error) {

	client, err := l.router.HtlcInterceptor(ctx)
	if err != nil {
		return nil, err
	}

	return &lndHtlcInterceptorClient{client: client}, nil
}

func (l *lndclientGrpc) Close() {
	l.conn.Close()
}

func (l *lndclientGrpc) getNodeAlias(key route.Vertex) (string, error) {
	ctx, cancel := context.WithTimeout(ctxb, rpcTimeout)
	defer cancel()

	l.log.Debugw("Retrieving node info",
		"key", key)

	info, err := l.main.GetNodeInfo(ctx, &lnrpc.NodeInfoRequest{
		PubKey: key.String(),
	})
	switch {
	case status.Code(err) == codes.NotFound:
		return "", ErrNodeNotFound

	case info.Node == nil:
		return "", ErrNodeNotFound

	case err != nil:
		return "", err
	}

	return info.Node.Alias, nil
}

func (l *lndclientGrpc) getPendingIncomingHtlcs(ctx context.Context, peer *route.Vertex) (
	map[route.Vertex]map[circuitKey]*inFlightHtlc, error) {

	ctx, cancel := context.WithTimeout(ctx, rpcTimeout)
	defer cancel()

	req := &lnrpc.ListChannelsRequest{}
	if peer != nil {
		req.Peer = peer[:]
	}

	resp, err := l.main.ListChannels(ctx, req)
	if err != nil {
		return nil, err
	}

	allHtlcs := make(map[route.Vertex]map[circuitKey]*inFlightHtlc)
	for _, channel := range resp.Channels {
		peer, err := route.NewVertexFromStr(channel.RemotePubkey)
		if err != nil {
			return nil, err
		}

		htlcs, ok := allHtlcs[peer]
		if !ok {
			htlcs = make(map[circuitKey]*inFlightHtlc)
			allHtlcs[peer] = htlcs
		}

		for _, htlc := range channel.PendingHtlcs {
			if !htlc.Incoming {
				continue
			}

			key := circuitKey{
				channel: channel.ChanId,
				htlc:    htlc.HtlcIndex,
			}

			// Note: we cannot easily recover added timestamp or incoming
			// and outgoing amounts on resume, so we leave these values as
			// zero to indicate that they are unknown due to restart.
			htlcs[key] = &inFlightHtlc{}
		}
	}

	return allHtlcs, nil
}
