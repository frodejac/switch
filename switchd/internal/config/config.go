package config

import (
	"fmt"
	"time"
)

// ServerConfig holds all configuration for the server
type ServerConfig struct {
	// Node configuration
	Node struct {
		ID string `json:"id"`
	} `json:"node"`

	// HTTP server configuration
	HTTP struct {
		Address string `json:"address"`
	} `json:"http"`

	// Raft configuration
	Raft struct {
		Address       string        `json:"address"`
		AdvertiseAddr string        `json:"advertise_addr"`
		Directory     string        `json:"directory"`
		Bootstrap     bool          `json:"bootstrap"`
		JoinAddress   string        `json:"join_address"`
		JoinTimeout   time.Duration `json:"join_timeout"`
	} `json:"raft"`

	// Storage configuration
	Storage struct {
		// FeatureFlags is the directory for feature flag storage
		FeatureFlags string `json:"feature_flags"`
		// Membership is the directory for membership storage
		Membership string `json:"membership"`
	} `json:"storage"`

	// Feature configuration
	Features struct {
		PreWarmRules bool `json:"pre_warm_rules"`
	} `json:"features"`
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
		return fmt.Errorf("Raft address is required")
	}

	if c.Raft.Directory == "" {
		return fmt.Errorf("Raft directory is required")
	}

	// Set default storage directories if not specified
	if c.Storage.FeatureFlags == "" {
		c.Storage.FeatureFlags = "data/feature_flags"
	}
	if c.Storage.Membership == "" {
		c.Storage.Membership = "data/membership"
	}

	return nil
}

// New creates a new server configuration with default values
func New() *ServerConfig {
	return &ServerConfig{
		Raft: struct {
			Address       string        `json:"address"`
			AdvertiseAddr string        `json:"advertise_addr"`
			Directory     string        `json:"directory"`
			Bootstrap     bool          `json:"bootstrap"`
			JoinAddress   string        `json:"join_address"`
			JoinTimeout   time.Duration `json:"join_timeout"`
		}{
			JoinTimeout: 10 * time.Second,
		},
	}
}
