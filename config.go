package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
	"gopkg.in/yaml.v2"
)

type yamlGroupConfig struct {
	MaxPendingHtlcs int           `yaml:"maxPendingHtlcs"`
	HtlcMinInterval time.Duration `yaml:"htlcMinInterval"`
	HtlcBurstSize   int           `yaml:"htlcBurstSize"`
}

type yamlGroup struct {
	yamlGroupConfig `yaml:",inline"`

	Peers []string `yaml:"peers"`
}

type yamlConfig struct {
	yamlGroupConfig `yaml:",inline"`

	Groups  []yamlGroup `yaml:"groups"`
	HoldFee holdFee     `yaml:"holdFee"`
}

type holdFee struct {
	BaseSatPerHr      int64       `yaml:"baseSatPerHr"`
	RatePpmPerHr      int         `yaml:"ratePpmPerHr"`
	ReportingInterval yamlTimeDur `yaml:"reportingInterval"`
}

type yamlTimeDur time.Duration

func (t *yamlTimeDur) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var tm string
	if err := unmarshal(&tm); err != nil {
		return err
	}

	td, err := time.ParseDuration(tm)
	if err != nil {
		return fmt.Errorf("failed to parse '%s' to time.Duration: %v", tm, err)
	}

	*t = yamlTimeDur(td)
	return nil
}

func (t *yamlTimeDur) Duration() time.Duration {
	return time.Duration(*t)
}

type groupConfig struct {
	MaxPendingHtlcs int

	HtlcMinInterval time.Duration
	HtlcBurstSize   int
}

type config struct {
	groupConfig

	PerPeer map[route.Vertex]groupConfig

	BaseSatPerHr      int64
	RatePpmPerHr      int
	ReportingInterval time.Duration
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

	yamlFile, err := ioutil.ReadFile(c.path)
	if err != nil {
		return nil, err
	}

	var yamlCfg yamlConfig
	err = yaml.UnmarshalStrict(yamlFile, &yamlCfg)
	if err != nil {
		return nil, err
	}

	parseGroupConfig := func(cfg *yamlGroupConfig) groupConfig {
		burstSize := cfg.HtlcBurstSize
		if burstSize == 0 {
			burstSize = 1
		}

		return groupConfig{
			MaxPendingHtlcs: cfg.MaxPendingHtlcs,
			HtlcMinInterval: cfg.HtlcMinInterval,
			HtlcBurstSize:   burstSize,
		}
	}

	config := config{
		groupConfig:       parseGroupConfig(&yamlCfg.yamlGroupConfig),
		PerPeer:           make(map[route.Vertex]groupConfig),
		BaseSatPerHr:      yamlCfg.HoldFee.BaseSatPerHr,
		RatePpmPerHr:      yamlCfg.HoldFee.RatePpmPerHr,
		ReportingInterval: time.Duration(yamlCfg.HoldFee.ReportingInterval),
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

			config.PerPeer[peerPubkey] = parseGroupConfig(
				&group.yamlGroupConfig,
			)
		}
	}

	log.Infow("Read config file",
		"file", c.path)

	return &config, nil
}
