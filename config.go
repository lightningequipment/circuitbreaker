package main

import (
	"fmt"
	"os"
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
	"gopkg.in/yaml.v2"
)

type yamlGroupConfig struct {
	MaxPendingHtlcs int           `yaml:"maxPendingHtlcs"`
	HtlcMinInterval time.Duration `yaml:"htlcMinInterval"`
	HtlcBurstSize   int           `yaml:"htlcBurstSize"`
	Mode            string        `yaml:"mode"`
}

type yamlGroup struct {
	yamlGroupConfig `yaml:",inline"`

	Peers []string `yaml:"peers"`
}

type yamlConfig struct {
	yamlGroupConfig `yaml:",inline"`

	Groups []yamlGroup `yaml:"groups"`
}

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

type groupConfig struct {
	MaxPendingHtlcs int

	HtlcMinInterval time.Duration
	HtlcBurstSize   int

	Mode Mode
}

type config struct {
	groupConfig

	PerPeer map[route.Vertex]groupConfig
}

// forPeer returns the config for a specific peer.
func (c *config) forPeer(peer route.Vertex) *groupConfig {
	if cfg, ok := c.PerPeer[peer]; ok {
		return &cfg
	}

	return &c.groupConfig
}

type configLoader struct {
	path string
}

func newConfigLoader(path string) *configLoader {
	loader := &configLoader{
		path: path,
	}

	return loader
}

func (c *configLoader) load() (*config, error) {
	if _, err := os.Stat(c.path); os.IsNotExist(err) {
		return nil, fmt.Errorf("no config file at %v", c.path)
	}

	yamlFile, err := os.ReadFile(c.path)
	if err != nil {
		return nil, err
	}

	var yamlCfg yamlConfig
	err = yaml.UnmarshalStrict(yamlFile, &yamlCfg)
	if err != nil {
		return nil, err
	}

	parseGroupConfig := func(cfg *yamlGroupConfig) (groupConfig, error) {
		burstSize := cfg.HtlcBurstSize
		if burstSize == 0 {
			burstSize = 1
		}

		var mode Mode
		switch cfg.Mode {
		case "", "fail":
			mode = ModeFail

		case "queue":
			mode = ModeQueue

		case "queue_peer_initiated":
			mode = ModeQueuePeerInitiated

		default:
			return groupConfig{}, fmt.Errorf("unknown mode %v", cfg.Mode)
		}

		return groupConfig{
			MaxPendingHtlcs: cfg.MaxPendingHtlcs,
			HtlcMinInterval: cfg.HtlcMinInterval,
			HtlcBurstSize:   burstSize,
			Mode:            mode,
		}, nil
	}

	defaultCfg, err := parseGroupConfig(&yamlCfg.yamlGroupConfig)
	if err != nil {
		return nil, err
	}

	config := config{
		groupConfig: defaultCfg,
		PerPeer:     make(map[route.Vertex]groupConfig),
	}

	for _, group := range yamlCfg.Groups {
		for _, peer := range group.Peers {
			peerPubkey, err := route.NewVertexFromStr(peer)
			if err != nil {
				return nil, err
			}

			_, exists := config.PerPeer[peerPubkey]
			if exists {
				return nil, fmt.Errorf("peer %v in multiple groups",
					peerPubkey)
			}

			peerCfg, err := parseGroupConfig(&group.yamlGroupConfig)
			if err != nil {
				return nil, err
			}

			config.PerPeer[peerPubkey] = peerCfg
		}
	}

	log.Infow("Read config file",
		"file", c.path)

	return &config, nil
}
