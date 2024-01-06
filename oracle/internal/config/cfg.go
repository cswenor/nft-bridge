package config

import (
	"flag"
	"fmt"
	"nft-bridge/internal/utils"
)

var cfgFile = flag.String("f", "config.jsonc", "config file")

type ServiceConfig struct {
	Address string `json:"address"`
	Port    string `json:"port"`
	Token   string `json:"token"`
}

type ChainConfig struct {
	Algod   ServiceConfig `json:"algod"`
	Indexer ServiceConfig `json:"indexer"`
}

type PKeys struct {
	Algorand string `json:"algorand"`
	Voi      string `json:"voi"`
}

type Chains struct {
	Algorand ChainConfig `json:"algorand"`
	Voi      ChainConfig `json:"voi"`
}

type BotConfig struct {
	ChainAPIs Chains `json:"chain-apis"`
	PKeys     PKeys  `json:"pkeys"`
}

var defaultConfig = BotConfig{}

// loadConfig loads the configuration from the specified file, merging into the default configuration.
func LoadConfig() (cfg BotConfig, err error) {
	flag.Parse()
	cfg = defaultConfig
	err = utils.LoadJSONCFromFile(*cfgFile, &cfg)

	// Check for Algorand and Voi configurations
	if cfg.ChainAPIs.Algorand.Algod.Address == "" || cfg.ChainAPIs.Algorand.Algod.Token == "" {
		return cfg, fmt.Errorf("[CFG] Incomplete Algorand Algod config")
	}

	if cfg.ChainAPIs.Algorand.Indexer.Address == "" || cfg.ChainAPIs.Algorand.Indexer.Token == "" {
		return cfg, fmt.Errorf("[CFG] Incomplete Algorand Indexer config")
	}

	if cfg.ChainAPIs.Voi.Algod.Address == "" || cfg.ChainAPIs.Voi.Algod.Token == "" {
		return cfg, fmt.Errorf("[CFG] Incomplete Voi Algod config")
	}

	if cfg.ChainAPIs.Voi.Indexer.Address == "" || cfg.ChainAPIs.Voi.Indexer.Token == "" {
		return cfg, fmt.Errorf("[CFG] Incomplete Voi Indexer config")
	}

	// Check for specific private keys
	if cfg.PKeys.Algorand == "" {
		return cfg, fmt.Errorf("[CFG] Missing 'algorand' private key in pkeys config")
	}
	if cfg.PKeys.Voi == "" {
		return cfg, fmt.Errorf("[CFG] Missing 'voi' private key in pkeys config")
	}

	return cfg, err
}
