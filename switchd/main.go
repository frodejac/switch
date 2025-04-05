package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/frodejac/switch/switchd/internal/config"
	"github.com/frodejac/switch/switchd/internal/logging"
	"github.com/frodejac/switch/switchd/internal/server"
)

func main() {
	// Initialize logging
	loglevel := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		loglevel = slog.LevelDebug
	}
	logging.Init(loglevel)

	// Load configuration from environment variables
	cfg, err := config.Load()
	if err != nil {
		logging.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	logging.Info("configuration", "config", cfg)

	// Parse command line flags (these will override environment variables)
	flag.StringVar(&cfg.Node.ID, "node-id", cfg.Node.ID, "Node ID for this server")
	flag.StringVar(&cfg.HTTP.Address, "http-addr", cfg.HTTP.Address, "HTTP server address")
	flag.StringVar(&cfg.Raft.Address, "raft-addr", cfg.Raft.Address, "Raft server address")
	flag.StringVar(&cfg.Raft.AdvertiseAddr, "raft-advertise-addr", cfg.Raft.AdvertiseAddr, "Raft advertised address")
	flag.StringVar(&cfg.Raft.Directory, "raft-dir", cfg.Raft.Directory, "Raft data directory")
	flag.BoolVar(&cfg.Raft.Bootstrap, "bootstrap", cfg.Raft.Bootstrap, "Bootstrap the cluster")
	flag.StringVar(&cfg.Raft.JoinAddress, "join", cfg.Raft.JoinAddress, "Address of leader node to join")
	flag.BoolVar(&cfg.Features.PreWarmRules, "pre-warm", cfg.Features.PreWarmRules, "Pre-warm the CEL cache on startup")
	flag.DurationVar(&cfg.Raft.JoinTimeout, "join-timeout", cfg.Raft.JoinTimeout, "Timeout for join and leader election operations")
	flag.StringVar(&cfg.Storage.FeatureFlags, "feature-flags-dir", cfg.Storage.FeatureFlags, "Directory for feature flag storage")
	flag.StringVar(&cfg.Storage.Membership, "membership-dir", cfg.Storage.Membership, "Directory for membership storage")
	flag.Parse()

	if err := cfg.Validate(); err != nil {
		logging.Error("failed to validate config", "error", err)
		os.Exit(1)
	}

	// Ensure data directories exist
	if err := os.MkdirAll(cfg.Raft.Directory, 0755); err != nil {
		logging.Error("failed to create raft directory", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.Storage.FeatureFlags, 0755); err != nil {
		logging.Error("failed to create feature flags directory", "error", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.Storage.Membership, 0755); err != nil {
		logging.Error("failed to create membership directory", "error", err)
		os.Exit(1)
	}

	srv, err := server.NewServer(cfg)
	if err != nil {
		logging.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	if err := srv.Start(); err != nil {
		logging.Error("failed to start server", "error", err)
		os.Exit(1)
	}

	// Wait for interrupt signal
	terminate := make(chan os.Signal, 1)
	signal.Notify(terminate, os.Interrupt, syscall.SIGTERM)
	<-terminate

	logging.Info("shutting down server...")
	if err := srv.Stop(); err != nil {
		logging.Error("failed to stop server", "error", err)
		os.Exit(1)
	}
}
