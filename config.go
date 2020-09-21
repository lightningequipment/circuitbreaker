package main

import (
	"fmt"
	"io/ioutil"
	"os"

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
}

type config struct {
	MaxPendingHtlcs        int
	MaxPendingHtlcsPerPeer map[route.Vertex]int
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

	config := config{
		MaxPendingHtlcs:        yamlCfg.MaxPendingHtlcs,
		MaxPendingHtlcsPerPeer: make(map[route.Vertex]int),
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

	log.Info("Read config file", zap.String("file", c.path))

	return &config, nil
}
