package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

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
	store, err := storage.NewBadgerStore(config.Storage.FeatureFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to create feature flag store: %v", err)
	}
	s.store = store

	// Initialize membership store
	membershipStore, err := storage.NewBadgerMembershipStore(config.Storage.Membership)
	if err != nil {
		store.Close()
		return nil, fmt.Errorf("failed to create membership store: %v", err)
	}
	s.membership = membershipStore

	// Initialize rules engine
	engine, err := rules.NewEngine()
	if err != nil {
		store.Close()
		membershipStore.Close()
		return nil, fmt.Errorf("failed to create rules engine: %v", err)
	}
	s.rules = engine
	s.cache = rules.NewCache()

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

	// Register routes
	e.GET("/:store/:key", s.handleGet)
	e.PUT("/:store/:key", s.handlePut)
	e.POST("/join", s.handleJoin)
	e.GET("/:store", s.handleList)
	e.GET("/stores", s.handleListStores)
	e.DELETE("/:store/:key", s.handleDelete)
	e.GET("/status", s.handleStatus)
	s.httpServer = e

	return s, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Initialize Raft
	if err := s.setupRaft(); err != nil {
		return fmt.Errorf("failed to setup raft: %v", err)
	}

	// If we have a join address, try to join the cluster
	if s.config.Raft.JoinAddress != "" {
		if err := s.JoinCluster(s.config.Raft.JoinAddress); err != nil {
			return fmt.Errorf("failed to join cluster: %v", err)
		}
	}

	if s.config.Features.PreWarmRules {
		if err := s.preWarmCache(); err != nil {
			return fmt.Errorf("failed to pre-warm cache: %v", err)
		}
	}

	// Start HTTP server
	go func() {
		if err := s.httpServer.Start(s.config.HTTP.Address); err != nil && err != http.ErrServerClosed {
			logging.Error("failed to start HTTP server", "error", err)
			os.Exit(1)
		}
	}()

	return nil
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
	if err := s.httpServer.Close(); err != nil {
		return fmt.Errorf("failed to stop HTTP server: %v", err)
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

	return nil
}
