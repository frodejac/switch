package config

import (
	"fmt"
	"time"
)

// ServerConfig holds the complete server configuration
type ServerConfig struct {
	Node     NodeConfig
	HTTP     HTTPConfig
	Raft     RaftConfig
	Storage  StorageConfig
	Features FeatureConfig
}

// NodeConfig holds node-specific configuration
type NodeConfig struct {
	ID      string
	PreWarm bool // Whether to pre-warm the CEL cache on startup
}

// HTTPConfig holds HTTP server configuration
type HTTPConfig struct {
	Address string
}

// RaftConfig holds Raft-specific configuration
type RaftConfig struct {
	Address       string
	AdvertiseAddr string
	Directory     string
	Bootstrap     bool
	JoinAddress   string
	JoinTimeout   time.Duration
}

// StorageConfig holds storage-specific configuration
type StorageConfig struct {
	Directory string
}

// FeatureConfig holds feature-specific configuration
type FeatureConfig struct {
	PreWarmRules bool
}

// New creates a new server configuration with default values
func New() *ServerConfig {
	return &ServerConfig{
		Node: NodeConfig{
			PreWarm: false,
		},
		Raft: RaftConfig{
			JoinTimeout: 5 * time.Second,
		},
	}
}

// Validate performs validation of the configuration
func (c *ServerConfig) Validate() error {
	if c.Node.ID == "" {
		return fmt.Errorf("node ID is required")
	}
	if c.HTTP.Address == "" {
		return fmt.Errorf("HTTP address is required")
	}
	if c.Raft.Address == "" {
		return fmt.Errorf("Raft address is required")
	}
	if c.Raft.Directory == "" {
		return fmt.Errorf("Raft directory is required")
	}
	return nil
}
