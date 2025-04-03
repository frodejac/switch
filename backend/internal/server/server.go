package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/cel-go/cel"
	"github.com/hashicorp/raft"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/frodejac/switch/internal/config"
	"github.com/frodejac/switch/internal/logging"
)

// Config holds the server configuration
type Config struct {
	NodeID            string
	HTTPAddr          string
	RaftAddr          string
	RaftAdvertiseAddr string // Advertised Raft address for other nodes to connect to
	RaftDir           string
	Bootstrap         bool
	PreWarm           bool          // Whether to pre-warm the CEL cache on startup
	JoinTimeout       time.Duration // Timeout for join and leader election operations
}

// Server represents our feature flag server
type Server struct {
	config     *config.ServerConfig
	store      *badger.DB
	raft       *raft.Raft
	httpServer *echo.Echo
	celEnv     *cel.Env

	// CEL program cache
	celCache sync.Map
}

// MembershipEntry represents a node's metadata in the membership register
type MembershipEntry struct {
	NodeID      string `json:"node_id"`
	RaftAddr    string `json:"raft_addr"`
	HTTPAddr    string `json:"http_addr"`
	LastUpdated int64  `json:"last_updated"`
}

// membershipKey returns the BadgerDB key for a node's membership entry
func membershipKey(nodeID string) []byte {
	return []byte(fmt.Sprintf("__membership/%s", nodeID))
}

// GetMembership returns the membership information for a node
func (s *Server) GetMembership(nodeID string) (*MembershipEntry, error) {
	var entry MembershipEntry
	err := s.store.View(func(txn *badger.Txn) error {
		item, err := txn.Get(membershipKey(nodeID))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry)
		})
	})
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetLeaderHTTPAddr returns the HTTP address of the current leader
func (s *Server) GetLeaderHTTPAddr() (string, error) {
	_, leaderID := s.raft.LeaderWithID()
	if leaderID == "" {
		return "", fmt.Errorf("no leader found")
	}

	logging.Info("fetching leader membership data", "leader", leaderID)

	// Get membership information for the leader
	membership, err := s.GetMembership(string(leaderID))
	if err != nil {
		return "", fmt.Errorf("failed to get leader membership: %v", err)
	}

	return membership.HTTPAddr, nil
}

// NewServer creates a new server instance
func NewServer(config *config.ServerConfig) (*Server, error) {
	s := &Server{
		config: config,
	}

	// Initialize BadgerDB
	opts := badger.DefaultOptions(filepath.Join(config.Raft.Directory, "badger"))
	opts.Logger = logging.NewBadgerLogger(logging.Logger)
	store, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger store: %v", err)
	}
	s.store = store

	// Initialize CEL environment
	env, err := cel.NewEnv(
		cel.Variable("key", cel.StringType),
		cel.Variable("context", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("request", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("device", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("time", cel.TimestampType),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %v", err)
	}
	s.celEnv = env

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
	s.httpServer = e

	return s, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Initialize Raft
	if err := s.setupRaft(); err != nil {
		return fmt.Errorf("failed to setup raft: %v", err)
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

// Stop gracefully stops the server
func (s *Server) Stop() error {
	if err := s.httpServer.Close(); err != nil {
		return fmt.Errorf("failed to stop HTTP server: %v", err)
	}

	if s.raft != nil {
		s.raft.Shutdown()
	}

	if s.store != nil {
		if err := s.store.Close(); err != nil {
			return fmt.Errorf("failed to close badger store: %v", err)
		}
	}

	return nil
}
