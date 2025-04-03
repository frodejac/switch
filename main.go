package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/frodejac/switch/internal/logging"
	"github.com/frodejac/switch/internal/server"
)

func main() {
	// Initialize logging
	logging.Init(slog.LevelInfo)

	// Parse command line flags
	nodeID := flag.String("node-id", "", "Node ID for this server")
	httpAddr := flag.String("http-addr", "0.0.0.0:9990", "HTTP server address")
	raftAddr := flag.String("raft-addr", "0.0.0.0:9991", "Raft server address")
	raftAdvertiseAddr := flag.String("raft-advertise-addr", "", "Raft advertised address")
	raftDir := flag.String("raft-dir", "data", "Raft data directory")
	bootstrap := flag.Bool("bootstrap", false, "Bootstrap the cluster")
	join := flag.String("join", "", "Address of leader node to join")
	preWarm := flag.Bool("pre-warm", false, "Pre-warm the CEL cache on startup")
	joinTimeout := flag.Duration("join-timeout", 10*time.Second, "Timeout for join and leader election operations")
	flag.Parse()

	if *nodeID == "" {
		logging.Error("node-id is required")
		os.Exit(1)
	}

	// Ensure data directories exist
	if err := os.MkdirAll(*raftDir, 0755); err != nil {
		logging.Error("failed to create data directory", "error", err)
		os.Exit(1)
	}

	// Create and start server
	config := &server.Config{
		NodeID:            *nodeID,
		HTTPAddr:          *httpAddr,
		RaftAddr:          *raftAddr,
		RaftAdvertiseAddr: *raftAdvertiseAddr,
		RaftDir:           *raftDir,
		Bootstrap:         *bootstrap,
		PreWarm:           *preWarm,
		JoinTimeout:       *joinTimeout,
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
	if *join != "" {
		if err := srv.Join(*join); err != nil {
			logging.Error("failed to join cluster", "address", *join, "error", err)
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
