package main

import (
	"context"
	"crypto/sha256"
	"errors"
	"math/rand"
	"sync"
	"time"

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

type stubLndClient struct {
	peers   map[route.Vertex]*stubPeer
	chanMap map[uint64]route.Vertex

	pendingHtlcs     map[route.Vertex]map[circuitKey]struct{}
	pendingHtlcsLock sync.Mutex

	eventChan             chan *resolvedEvent
	interceptRequestChan  chan *interceptedEvent
	interceptResponseChan chan *interceptResponse
}

type stubPeer struct {
	channels map[uint64]*stubChannel
	alias    string
}

func newStubClient() *stubLndClient {
	peers := make(map[route.Vertex]*stubPeer)
	chanMap := make(map[uint64]route.Vertex)
	pendingHtlcs := make(map[route.Vertex]map[circuitKey]struct{})

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

		pendingHtlcs[key] = make(map[circuitKey]struct{})

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

		go client.generateHtlcs(key, peer)
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
			circuitKey: resp.key,
			settled:    false,
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
		circuitKey: resp.key,
		settled:    settled,
	}

	s.pendingHtlcsLock.Lock()
	delete(s.pendingHtlcs[key], resp.key)
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

func (s *stubLndClient) generateHtlcs(key route.Vertex, peer *stubPeer) {
	log.Infow("Starting stub", "peer", peer.alias)
	chanCount := len(peer.channels)
	chanIds := make([]uint64, 0)
	for chanId := range peer.channels {
		chanIds = append(chanIds, chanId)
	}

	delayProfile := int(key[5] % 3)

	var htlcId uint64
	for {
		chanId := chanIds[rand.Int31n(int32(chanCount))] //nolint: gosec

		circuitKey := circuitKey{
			channel: chanId,
			htlc:    htlcId,
		}

		s.pendingHtlcsLock.Lock()
		s.pendingHtlcs[key][circuitKey] = struct{}{}
		s.pendingHtlcsLock.Unlock()

		s.interceptRequestChan <- &interceptedEvent{
			circuitKey: circuitKey,
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

func (s *stubLndClient) getPendingIncomingHtlcs(ctx context.Context, peer *route.Vertex) (
	map[route.Vertex]map[circuitKey]struct{}, error) {

	allHtlcs := make(map[route.Vertex]map[circuitKey]struct{})

	s.pendingHtlcsLock.Lock()
	for peerKey, peerHtlcs := range s.pendingHtlcs {
		if peer != nil && peerKey != *peer {
			continue
		}

		htlcs := make(map[circuitKey]struct{})
		for htlc := range peerHtlcs {
			htlcs[htlc] = struct{}{}
		}

		allHtlcs[peerKey] = htlcs
	}
	s.pendingHtlcsLock.Unlock()

	return allHtlcs, nil
}
