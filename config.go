package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/lightningnetwork/lnd/routing/route"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"
)

type yamlGroup struct {
	MaxPendingHtlcs int      `yaml:"maxPendingHtlcs"`
	Peers           []string `yaml:"peers"`
}

type yamlConfig struct {
	MaxPendingHtlcs int         `yaml:"maxPendingHtlcs"`
	Groups          []yamlGroup `yaml:"groups"`
	HoldFee         holdFee     `yaml:"holdFee"`
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

type config struct {
	MaxPendingHtlcs        int
	MaxPendingHtlcsPerPeer map[route.Vertex]int

	BaseSatPerHr      int64
	RatePpmPerHr      int
	ReportingInterval time.Duration
}

var defaultConfig = config{
	MaxPendingHtlcs: 1,
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
		log.Debug("No config file, using defaults",
			zap.String("file", c.path))

		return &defaultConfig, nil
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

	if yamlCfg.HoldFee.ReportingInterval == 0 {
		return nil, errors.New("reportingInterval not set")
	}

	config := config{
		MaxPendingHtlcs:        yamlCfg.MaxPendingHtlcs,
		MaxPendingHtlcsPerPeer: make(map[route.Vertex]int),
		BaseSatPerHr:           yamlCfg.HoldFee.BaseSatPerHr,
		RatePpmPerHr:           yamlCfg.HoldFee.RatePpmPerHr,
		ReportingInterval:      time.Duration(yamlCfg.HoldFee.ReportingInterval),
	}

	for _, group := range yamlCfg.Groups {
		for _, peer := range group.Peers {
			peerPubkey, err := route.NewVertexFromStr(peer)
			if err != nil {
				return nil, err
			}

			_, exists := config.MaxPendingHtlcsPerPeer[peerPubkey]
			if exists {
				return nil, fmt.Errorf("peer %v in multiple groups",
					peerPubkey)
			}

			config.MaxPendingHtlcsPerPeer[peerPubkey] =
				group.MaxPendingHtlcs
		}
	}

	log.Infow("Read config file",
		"file", c.path)

	return &config, nil
}
