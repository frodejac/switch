package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/raft"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/frodejac/switch/switchd/internal/api"
	"github.com/frodejac/switch/switchd/internal/config"
	"github.com/frodejac/switch/switchd/internal/consensus"
	"github.com/frodejac/switch/switchd/internal/logging"
	"github.com/frodejac/switch/switchd/internal/rules"
	"github.com/frodejac/switch/switchd/internal/storage"
)

// Server represents our feature flag server
type Server struct {
	config     *config.ServerConfig
	store      storage.Store
	ffStore    storage.FeatureFlagStore
	membership storage.MembershipStore
	raft       *consensus.RaftNode
	httpServer *echo.Echo
	rules      *rules.Engine
	cache      *rules.Cache
}

// NewServer creates a new server instance
func NewServer(config *config.ServerConfig) (*Server, error) {
	s := &Server{
		config: config,
	}

	// Initialize feature flag store
	store, err := storage.NewBadgerFeatureFlagStore(config.Storage.FeatureFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create feature flag store: %v", err)
	}
	s.store = store
	s.ffStore = store

	// Initialize membership store
	membershipStore, err := storage.NewBadgerMembershipStore(config.Storage.Membership)
	if err != nil {
		s.Stop()
		return nil, fmt.Errorf("failed to create membership store: %v", err)
	}
	s.membership = membershipStore

	// Initialize rules engine
	engine, err := rules.NewEngine()
	if err != nil {
		s.Stop()
		return nil, fmt.Errorf("failed to create rules engine: %v", err)
	}
	s.rules = engine
	s.cache = rules.NewCache()

	// Initialize Raft
	raftNode, err := consensus.NewRaftNode(s.config, s.store, s.membership)
	if err != nil {
		s.Stop()
		return nil, fmt.Errorf("failed to create raft node: %v", err)
	}
	s.raft = raftNode

	// If we're bootstrapping, wait for this node to become leader
	if s.config.Raft.Bootstrap {
		timeout := time.After(5 * time.Second)
		for {
			if s.raft.GetState() == raft.Leader {
				break
			}
			select {
			case <-timeout:
				logging.Error("timeout waiting for node to become leader")
				s.Stop()
				return nil, fmt.Errorf("timeout waiting for node to become leader")
			case <-time.After(100 * time.Millisecond):
			}
		}

	}

	logging.Info("raft node initialized", "state", s.raft.GetState())

	// Initialize HTTP server
	e := echo.New()
	e.Logger = logging.NewEchoLogger(logging.Logger)
	e.Use(logging.LoggerMiddleware())
	e.Use(middleware.Recover())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"*"},
		AllowMethods: []string{http.MethodGet, http.MethodPut, http.MethodPost, http.MethodDelete},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept},
	}))

	router := api.NewRouter(s.store, s.ffStore, s.rules, s.cache, s.raft, s.membership)
	router.SetupRoutes(e)
	s.httpServer = e

	return s, nil
}

// Start starts the server
func (s *Server) Start() error {
	// If we have a join address, try to join the cluster
	if s.config.Raft.JoinAddress != "" {
		logging.Info("attempting to join cluster", "address", s.config.Raft.JoinAddress)
		if err := s.raft.Join(s.config.Raft.JoinAddress); err != nil {
			return fmt.Errorf("failed to join cluster: %v", err)
		}
		logging.Info("joined cluster", "address", s.config.Raft.JoinAddress, "state", s.raft.GetState(), "leader", s.raft.GetLeaderId(), "id", s.raft.GetId())
	}

	if s.config.Features.PreWarmRules {
		if err := s.preWarmCache(); err != nil {
			return fmt.Errorf("failed to pre-warm cache: %v", err)
		}
	}

	// Start HTTP server
	go func() {
		logging.Info("starting HTTP server", "address", s.config.HTTP.Address)
		if err := s.httpServer.Start(s.config.HTTP.Address); err != nil && err != http.ErrServerClosed {
			logging.Error("failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	return nil
}

// GetRaftState returns the current state of the Raft node
func (s *Server) GetRaftState() raft.RaftState {
	return s.raft.GetState()
}

// preWarmCache loads all flags and pre-compiles their CEL expressions
func (s *Server) preWarmCache() error {
	values, err := s.store.ListWithValues(context.Background(), "")
	if err != nil {
		return err
	}

	for key, value := range values {
		var flagData struct {
			Value      any    `json:"value"`
			Expression string `json:"expression,omitempty"`
		}
		if err := json.Unmarshal(value, &flagData); err != nil {
			continue // Skip invalid entries
		}

		// Skip if no expression
		if flagData.Expression == "" {
			continue
		}

		// Compile and cache the program
		program, err := s.rules.Compile(flagData.Expression)
		if err != nil {
			continue // Skip invalid expressions
		}

		// Store in cache
		s.cache.Set(key, program)
	}

	return nil
}

// Stop gracefully stops the server
func (s *Server) Stop() error {
	logging.Info("stopping server")
	if s.httpServer != nil {
		if err := s.httpServer.Close(); err != nil {
			return fmt.Errorf("failed to stop HTTP server: %v", err)
		}
	}

	if s.raft != nil {
		if err := s.raft.Shutdown(); err != nil {
			return fmt.Errorf("failed to stop raft node: %v", err)
		}
	}

	if s.store != nil {
		if err := s.store.Close(); err != nil {
			return fmt.Errorf("failed to close feature flag store: %v", err)
		}
	}

	if s.membership != nil {
		if err := s.membership.Close(); err != nil {
			return fmt.Errorf("failed to close membership store: %v", err)
		}
	}

	logging.Info("server stopped")
	return nil
}
