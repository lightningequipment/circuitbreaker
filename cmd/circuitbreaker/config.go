package main

import (
	"fmt"
	"os"
	"time"

	"github.com/lightningequipment/circuitbreaker"
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

type configLoader struct {
	path string
}

func newConfigLoader(path string) *configLoader {
	loader := &configLoader{
		path: path,
	}

	return loader
}

func (c *configLoader) load() (*circuitbreaker.Config, error) {
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

	parseGroupConfig := func(cfg *yamlGroupConfig) (circuitbreaker.GroupConfig, error) {
		burstSize := cfg.HtlcBurstSize
		if burstSize == 0 {
			burstSize = 1
		}

		var mode circuitbreaker.Mode
		switch cfg.Mode {
		case "", "fail":
			mode = circuitbreaker.ModeFail

		case "queue":
			mode = circuitbreaker.ModeQueue

		case "queue_peer_initiated":
			mode = circuitbreaker.ModeQueuePeerInitiated

		default:
			return circuitbreaker.GroupConfig{}, fmt.Errorf("unknown mode %v", cfg.Mode)
		}

		return circuitbreaker.GroupConfig{
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

	config := circuitbreaker.Config{
		GroupConfig: defaultCfg,
		PerPeer:     make(map[route.Vertex]circuitbreaker.GroupConfig),
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
