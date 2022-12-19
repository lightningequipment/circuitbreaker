package circuitbreaker

import (
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
)

type Mode int

const (
	ModeFail Mode = iota
	ModeQueue
	ModeQueuePeerInitiated
)

func (m Mode) String() string {
	switch m {
	case ModeFail:
		return "fail"

	case ModeQueue:
		return "queue"

	case ModeQueuePeerInitiated:
		return "queue_peer_initiated"

	default:
		panic("unknown mode")
	}
}

type GroupConfig struct {
	MaxPendingHtlcs int

	HtlcMinInterval time.Duration
	HtlcBurstSize   int

	Mode Mode
}

type Config struct {
	GroupConfig

	PerPeer map[route.Vertex]GroupConfig
}

// forPeer returns the config for a specific peer.
func (c *Config) forPeer(peer route.Vertex) *GroupConfig {
	if cfg, ok := c.PerPeer[peer]; ok {
		return &cfg
	}

	return &c.GroupConfig
}
