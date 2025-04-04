package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frodejac/switch/internal/config"
	"github.com/frodejac/switch/internal/logging"
	"github.com/frodejac/switch/internal/server"
)

func main() {
	// Initialize logging
	loglevel := slog.LevelInfo
	if os.Getenv("DEBUG") != "" {
		loglevel = slog.LevelDebug
	}
	logging.Init(loglevel)

	// Parse command line flags
	cfg := &config.ServerConfig{}

	flag.StringVar(&cfg.Node.ID, "node-id", "", "Node ID for this server")
	flag.StringVar(&cfg.HTTP.Address, "http-addr", "0.0.0.0:9990", "HTTP server address")
	flag.StringVar(&cfg.Raft.Address, "raft-addr", "0.0.0.0:9991", "Raft server address")
	flag.StringVar(&cfg.Raft.AdvertiseAddr, "raft-advertise-addr", "", "Raft advertised address")
	flag.StringVar(&cfg.Raft.Directory, "raft-dir", "data", "Raft data directory")
	flag.BoolVar(&cfg.Raft.Bootstrap, "bootstrap", false, "Bootstrap the cluster")
	flag.StringVar(&cfg.Raft.JoinAddress, "join", "", "Address of leader node to join")
	flag.BoolVar(&cfg.Features.PreWarmRules, "pre-warm", false, "Pre-warm the CEL cache on startup")
	flag.DurationVar(&cfg.Raft.JoinTimeout, "join-timeout", 10*time.Second, "Timeout for join and leader election operations")
	flag.StringVar(&cfg.Storage.FeatureFlags, "feature-flags-dir", "data/feature_flags", "Directory for feature flag storage")
	flag.StringVar(&cfg.Storage.Membership, "membership-dir", "data/membership", "Directory for membership storage")
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
