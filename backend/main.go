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
	logging.Init(slog.LevelInfo)

	// Parse command line flags

	config := &config.ServerConfig{}

	flag.StringVar(&config.Node.ID, "node-id", "", "Node ID for this server")
	flag.StringVar(&config.HTTP.Address, "http-addr", "0.0.0.0:9990", "HTTP server address")
	flag.StringVar(&config.Raft.Address, "raft-addr", "0.0.0.0:9991", "Raft server address")
	flag.StringVar(&config.Raft.AdvertiseAddr, "raft-advertise-addr", "", "Raft advertised address")
	flag.StringVar(&config.Raft.Directory, "raft-dir", "data", "Raft data directory")
	flag.BoolVar(&config.Raft.Bootstrap, "bootstrap", false, "Bootstrap the cluster")
	flag.StringVar(&config.Raft.JoinAddress, "join", "", "Address of leader node to join")
	flag.BoolVar(&config.Features.PreWarmRules, "pre-warm", false, "Pre-warm the CEL cache on startup")
	flag.DurationVar(&config.Raft.JoinTimeout, "join-timeout", 10*time.Second, "Timeout for join and leader election operations")
	flag.Parse()

	if err := config.Validate(); err != nil {
		logging.Error("failed to validate config", "error", err)
		os.Exit(1)
	}

	// Ensure data directories exist
	// TODO: This should be done in the server constructor
	if err := os.MkdirAll(config.Raft.Directory, 0755); err != nil {
		logging.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	srv, err := server.NewServer(config)
	if err != nil {
		logging.Error("failed to create server", "error", err)
		os.Exit(1)
	}

	if err := srv.Start(); err != nil {
		logging.Error("failed to start server", "error", err)
		os.Exit(1)
	}

	// If join address specified, attempt to join the cluster
	if config.Raft.JoinAddress != "" {
		if err := srv.Join(config.Raft.JoinAddress); err != nil {
			logging.Error("failed to join cluster", "address", config.Raft.JoinAddress, "error", err)
			os.Exit(1)
		}
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
