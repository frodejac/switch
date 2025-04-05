package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// NodeConfig holds node-specific configuration
type NodeConfig struct {
	ID string `env:"ID"`
}

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
	Address string `env:"ADDRESS" envDefault:"0.0.0.0:9990"`
}

// RaftConfig holds Raft-specific configuration
type RaftConfig struct {
	Address       string        `env:"ADDRESS" envDefault:"0.0.0.0:9991"`
	AdvertiseAddr string        `env:"ADVERTISE_ADDR"`
	Directory     string        `env:"DIRECTORY" envDefault:"data"`
	Bootstrap     bool          `env:"BOOTSTRAP"`
	JoinAddress   string        `env:"JOIN_ADDRESS"`
	JoinTimeout   time.Duration `env:"JOIN_TIMEOUT" envDefault:"10s"`
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	FeatureFlags string `env:"FEATURE_FLAGS" envDefault:"data/feature_flags"`
	Membership   string `env:"MEMBERSHIP" envDefault:"data/membership"`
}

// FeatureConfig holds feature-specific configuration
type FeatureConfig struct {
	PreWarmRules bool `env:"PRE_WARM_RULES"`
}

// ServerConfig holds all configuration for the server
type ServerConfig struct {
	Node     NodeConfig    `envPrefix:"NODE_"`
	HTTP     HTTPConfig    `envPrefix:"HTTP_"`
	Raft     RaftConfig    `envPrefix:"RAFT_"`
	Storage  StorageConfig `envPrefix:"STORAGE_"`
	Features FeatureConfig `envPrefix:"FEATURES_"`
}

// Validate validates the configuration
func (c *ServerConfig) Validate() error {
	if c.Node.ID == "" {
		return fmt.Errorf("node ID is required")
	}

	if c.HTTP.Address == "" {
		return fmt.Errorf("HTTP address is required")
	}

	if c.Raft.Address == "" {
		return fmt.Errorf("raft address is required")
	}

	if c.Raft.Directory == "" {
		return fmt.Errorf("raft directory is required")
	}

	return nil
}

// Load loads the configuration from environment variables
func Load() (*ServerConfig, error) {
	cfg := &ServerConfig{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("failed to parse environment variables: %w", err)
	}

	return cfg, nil
}
