package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"math/rand"
	"sync"
	"time"

	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/routing/route"
)

var stubNodes = []string{
	"WalletOfSatoshi.com",
	"deezy.io",
	"🕊️ born to be free 🕊️",
	"c-otto.de",
	"BCash_Is_Trash",
	"1sats.com 🤩 0 fee",
	"NordicRails",
	"Stone Of Jordan",
	"Mushi",
	"Boltz",
	"CryptoChill",
	"ACINQ",
	"Lightning Labs",
	"Bottlepay",
	"NYDIG",
	"Bitrefill",
	"Lightning.Watch",
	"OpenNode",
	"LND2",
	"coincharge",
	"LightningJoule",
	"Mafia",
	"",
	"",
	"",
}

type stubChannel struct {
	initiator bool
}

type stubInFlight struct {
	incomingPeer route.Vertex
	keyOut       circuitKey
}

type stubLndClient struct {
	peers   map[route.Vertex]*stubPeer
	chanMap map[uint64]route.Vertex

	// Provides a globally unique htlc id for outgoing htlcs to prevent
	// duplicates when two sending channels pick the same outgoing channel.
	outgoingHtlcIndex     uint64
	outgoingHtlcIndexLock sync.Mutex

	pendingHtlcs     map[circuitKey]*stubInFlight
	pendingHtlcsLock sync.Mutex

	eventChan             chan *resolvedEvent
	interceptRequestChan  chan *interceptedEvent
	interceptResponseChan chan *interceptResponse
}

func (s *stubLndClient) nextOutgoingHtlcIndex() uint64 {
	s.outgoingHtlcIndexLock.Lock()
	defer s.outgoingHtlcIndexLock.Unlock()

	s.outgoingHtlcIndex++

	return s.outgoingHtlcIndex
}

type stubPeer struct {
	channels map[uint64]*stubChannel
	alias    string
}

func newStubClient() *stubLndClient {
	peers := make(map[route.Vertex]*stubPeer)
	chanMap := make(map[uint64]route.Vertex)
	pendingHtlcs := make(map[circuitKey]*stubInFlight)

	var chanId uint64 = 1
	for i, alias := range stubNodes {
		// Derive key.
		hash := sha256.Sum256([]byte{byte(i)})
		keySlice := append([]byte{0}, hash[:]...)
		key, err := route.NewVertexFromBytes(keySlice)
		if err != nil {
			panic(err)
		}

		// Derive channels.
		channels := make(map[uint64]*stubChannel)
		channelCount := int(key[5]%5) + 1
		for i := 0; i < channelCount; i++ {
			initiator := key[6+i]%2 == 0
			channels[chanId] = &stubChannel{
				initiator: initiator,
			}

			chanMap[chanId] = key

			chanId++
		}

		if _, exists := peers[key]; exists {
			panic("duplicate stub peer key")
		}

		peers[key] = &stubPeer{
			alias:    alias,
			channels: channels,
		}

		log.Infow("populated", "peer", alias, "key", key[:])
	}

	client := &stubLndClient{
		peers:   peers,
		chanMap: chanMap,

		eventChan:             make(chan *resolvedEvent),
		interceptRequestChan:  make(chan *interceptedEvent),
		interceptResponseChan: make(chan *interceptResponse),

		pendingHtlcs: pendingHtlcs,
	}

	first := true
	for key, peer := range peers {
		// The first peer is in-active.
		if first {
			first = false

			continue
		}

		key, peer := key, peer

		// Include all channels except the incoming peer's as possible outgoing
		// channels.
		var channels []uint64
		for channel, channelPeer := range chanMap {
			if channelPeer == key {
				continue
			}
			channels = append(channels, channel)
		}

		go client.generateHtlcs(key, peer, channels)
	}

	go client.run()

	return client
}

func (s *stubLndClient) run() {
	for resp := range s.interceptResponseChan {
		go s.resolveHtlc(resp)
	}
}

func (s *stubLndClient) resolveHtlc(resp *interceptResponse) {
	if !resp.resume {
		s.eventChan <- &resolvedEvent{
			incomingCircuitKey: resp.key,
			settled:            false,
			timestamp:          time.Now(),
		}

		return
	}

	// Retrieve node key.
	key, ok := s.chanMap[resp.key.channel]
	if !ok {
		panic("peer not found")
	}

	// Random delay according to profile.
	delayProfile := int(key[6] % 3)

	time.Sleep(randomDelay(delayProfile))

	// Random settlement according to profile.
	var settledPerc int32
	switch key[7] % 3 {
	case 0:
		settledPerc = 5

	case 1:
		settledPerc = 50

	case 2:
		settledPerc = 90
	}

	settled := rand.Int31n(100) < settledPerc //nolint: gosec

	s.eventChan <- &resolvedEvent{
		incomingCircuitKey: resp.key,
		settled:            settled,
		timestamp:          time.Now(),
	}

	s.pendingHtlcsLock.Lock()
	delete(s.pendingHtlcs, resp.key)
	s.pendingHtlcsLock.Unlock()
}

func randomDelay(profile int) time.Duration {
	var delayMinMs, delayMaxMs int32
	switch profile {
	case 0:
		delayMinMs, delayMaxMs = 100, 1000

	case 1:
		delayMinMs, delayMaxMs = 1000, 5000

	case 2:
		delayMinMs, delayMaxMs = 5000, 30000
	}

	return time.Duration(delayMinMs+rand.Int31n(delayMaxMs-delayMinMs)) * //nolint: gosec
		time.Millisecond
}

func (s *stubLndClient) generateHtlcs(key route.Vertex, peer *stubPeer,
	outgoingChannels []uint64) {

	log.Infow("Starting stub", "peer", peer.alias)
	outgoingChanCount := int32(len(outgoingChannels))
	chanCount := len(peer.channels)
	chanIds := make([]uint64, 0)
	for chanId := range peer.channels {
		chanIds = append(chanIds, chanId)
	}

	delayProfile := int(key[5] % 3)

	var htlcId uint64
	for {
		chanInId := chanIds[rand.Int31n(int32(chanCount))] //nolint: gosec

		circuitKeyIn := circuitKey{
			channel: chanInId,
			htlc:    htlcId,
		}

		circuitKeyOut := circuitKey{
			channel: outgoingChannels[rand.Int31n(outgoingChanCount)], //nolint: gosec
			htlc:    s.nextOutgoingHtlcIndex(),
		}

		s.pendingHtlcsLock.Lock()
		s.pendingHtlcs[circuitKeyIn] = &stubInFlight{
			incomingPeer: key,
			keyOut:       circuitKeyOut,
		}
		s.pendingHtlcsLock.Unlock()

		// Randomly pick a non-zero incoming amount, and an outgoing amount that
		// is less than or equal to our incoming amount (but not zero).
		incomingAmount := rand.Int63n(100_000_000) + 1 //nolint: gosec
		outgoingAmount := incomingAmount / 2
		if outgoingAmount == 0 {
			outgoingAmount = incomingAmount
		}

		s.interceptRequestChan <- &interceptedEvent{
			circuitKey:   circuitKeyIn,
			incomingMsat: lnwire.MilliSatoshi(incomingAmount),
			outgoingMsat: lnwire.MilliSatoshi(outgoingAmount),
		}

		htlcId++

		time.Sleep(randomDelay(delayProfile))
	}
}

func (s *stubLndClient) getInfo() (*info, error) {
	return &info{
		nodeKey: route.Vertex{1, 2, 3},
		version: "v1.0.0",
		alias:   "fake",
	}, nil
}

func (s *stubLndClient) listChannels() (map[uint64]*channel, error) {
	allChannels := make(map[uint64]*channel)
	for key, peer := range s.peers {
		for chanId, ch := range peer.channels {
			allChannels[chanId] = &channel{
				peer:      key,
				initiator: ch.initiator,
			}
		}
	}

	return allChannels, nil
}

func (s *stubLndClient) getNodeAlias(key route.Vertex) (string, error) {
	peer, ok := s.peers[key]
	if !ok {
		return "", nil
	}

	return peer.alias, nil
}

type stubHtlcEventsClient struct {
	parent *stubLndClient
}

func newStubHtlcEventsClient(parent *stubLndClient) *stubHtlcEventsClient {
	return &stubHtlcEventsClient{
		parent: parent,
	}
}

func (s *stubHtlcEventsClient) recv() (*resolvedEvent, error) {
	event, ok := <-s.parent.eventChan
	if !ok {
		return nil, errors.New("no more events")
	}

	return event, nil
}

func (s *stubLndClient) subscribeHtlcEvents(ctx context.Context) (htlcEventsClient, error) {
	return newStubHtlcEventsClient(s), nil
}

type stubHtlcInterceptorClient struct {
	parent *stubLndClient
}

func newStubHtlcInterceptorClient(parent *stubLndClient) *stubHtlcInterceptorClient {
	return &stubHtlcInterceptorClient{
		parent: parent,
	}
}

func (s *stubHtlcInterceptorClient) recv() (*interceptedEvent, error) {
	event, ok := <-s.parent.interceptRequestChan
	if !ok {
		return nil, errors.New("no more events")
	}

	return event, nil
}

func (s *stubHtlcInterceptorClient) send(resp *interceptResponse) error {
	s.parent.interceptResponseChan <- resp

	return nil
}

func (s *stubLndClient) htlcInterceptor(ctx context.Context) (htlcInterceptorClient, error) {
	return newStubHtlcInterceptorClient(s), nil
}

func (s *stubLndClient) getPendingIncomingHtlcs(ctx context.Context, targetPeer *route.Vertex) (
	map[route.Vertex]map[circuitKey]struct{}, error) {

	allHtlcs := make(map[route.Vertex]map[circuitKey]struct{})

	s.pendingHtlcsLock.Lock()
	for htlcKey, inFlight := range s.pendingHtlcs {
		if targetPeer != nil && inFlight.incomingPeer != *targetPeer {
			continue
		}

		htlcs, ok := allHtlcs[inFlight.incomingPeer]
		if !ok {
			htlcs = make(map[circuitKey]struct{})
		}

		htlcs[htlcKey] = struct{}{}
	}
	s.pendingHtlcsLock.Unlock()

	return allHtlcs, nil
}
