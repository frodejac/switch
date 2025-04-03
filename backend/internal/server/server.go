package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/cel-go/cel"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"google.golang.org/protobuf/types/known/structpb"

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
	config     *Config
	store      *badger.DB
	raft       *raft.Raft
	httpServer *echo.Echo
	celEnv     *cel.Env

	// Protects flag evaluation cache
	mu    sync.RWMutex
	cache map[string]interface{}

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
func NewServer(config *Config) (*Server, error) {
	s := &Server{
		config: config,
		cache:  make(map[string]interface{}),
	}

	// Initialize BadgerDB
	opts := badger.DefaultOptions(filepath.Join(config.RaftDir, "badger"))
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
		if err := s.httpServer.Start(s.config.HTTPAddr); err != nil && err != http.ErrServerClosed {
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

// Join joins an existing Raft cluster
func (s *Server) Join(addr string) error {
	logging.Info("attempting to join cluster", "address", addr)

	// Create join request with node metadata
	joinRequest := struct {
		NodeID            string `json:"node_id"`
		RaftAddr          string `json:"raft_addr"`
		HTTPAddr          string `json:"http_addr"`
		RaftAdvertiseAddr string `json:"raft_advertise_addr,omitempty"`
	}{
		NodeID:            s.config.NodeID,
		RaftAddr:          s.config.RaftAddr,
		HTTPAddr:          s.config.HTTPAddr,
		RaftAdvertiseAddr: s.config.RaftAdvertiseAddr,
	}

	// Marshal the join request
	reqBody, err := json.Marshal(joinRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal join request: %v", err)
	}

	// Send join request to the specified address
	url := fmt.Sprintf("http://%s/join", addr)
	logging.Info("sending join request", "url", url)

	// Retry the join request a few times
	var lastErr error
	for i := 0; i < 3; i++ {
		req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
		if err != nil {
			return fmt.Errorf("failed to create join request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			lastErr = err
			logging.Warn("join attempt failed", "attempt", i+1, "error", err)
			time.Sleep(time.Second)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			logging.Info("successfully sent join request")
			break
		}

		body, _ := io.ReadAll(resp.Body)
		lastErr = fmt.Errorf("failed to join cluster: %s", body)
		logging.Warn("join attempt failed", "attempt", i+1, "status", resp.StatusCode, "body", string(body))
		time.Sleep(time.Second)
	}

	if lastErr != nil {
		return fmt.Errorf("failed to join cluster after 3 attempts: %v", lastErr)
	}

	// Now that we've sent the join request, wait for our node to become part of the cluster
	timeout := time.After(s.config.JoinTimeout)
	for {
		// Check if we're part of the cluster by looking at the configuration
		future := s.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to get raft configuration: %v", err)
		}

		// Check if our node is in the configuration
		config := future.Configuration()
		logging.Debug("current cluster configuration", "config", config)
		for _, server := range config.Servers {
			if server.ID == raft.ServerID(s.config.NodeID) {
				logging.Info("successfully joined cluster")
				return nil
			}
		}

		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting to be added to cluster")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// validateStoreName checks if a store name is valid
func validateStoreName(store string) error {
	if store == "" {
		return fmt.Errorf("store name cannot be empty")
	}
	if strings.HasPrefix(store, "__") {
		return fmt.Errorf("store name cannot start with '__'")
	}
	return nil
}

// handleGet handles GET requests for feature flags
func (s *Server) handleGet(c echo.Context) error {
	store := c.Param("store")
	if err := validateStoreName(store); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	key := c.Param("key")

	// Get context values from query parameters
	context := make(map[string]interface{})
	for k, v := range c.QueryParams() {
		if len(v) > 0 {
			context[k] = v[0]
		}
	}

	// Read from BadgerDB
	var value []byte
	err := s.store.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("%s/%s", store, key)))
		if err != nil {
			return err
		}
		value, err = item.ValueCopy(nil)
		return err
	})

	if err == badger.ErrKeyNotFound {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "key not found"})
	} else if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Parse the stored value
	var flagData struct {
		Value      interface{} `json:"value"`
		Expression string      `json:"expression,omitempty"`
	}
	if err := json.Unmarshal(value, &flagData); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "invalid flag data"})
	}

	// If there's no expression, return the raw value
	if flagData.Expression == "" {
		return c.JSON(http.StatusOK, flagData.Value)
	}

	// Try to get compiled program from cache
	cacheKey := fmt.Sprintf("%s/%s", store, key)
	var program cel.Program
	if cached, ok := s.celCache.Load(cacheKey); ok {
		program = cached.(cel.Program)
	} else {
		// Compile and cache the program
		ast, issues := s.celEnv.Compile(flagData.Expression)
		if issues != nil && issues.Err() != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "invalid expression"})
		}

		var err error
		program, err = s.celEnv.Program(ast)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create program"})
		}

		s.celCache.Store(cacheKey, program)
	}

	// Convert context to protobuf struct
	contextStruct, err := structpb.NewStruct(context)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "invalid context"})
	}

	// Evaluate the expression
	result, _, err := program.Eval(map[string]interface{}{
		"key":     key,
		"context": contextStruct.AsMap(),
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "evaluation failed"})
	}

	return c.JSON(http.StatusOK, result)
}

// handlePut handles PUT requests for feature flags
func (s *Server) handlePut(c echo.Context) error {
	store := c.Param("store")
	if err := validateStoreName(store); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	key := c.Param("key")

	// If not leader, forward to leader immediately
	if s.raft.State() != raft.Leader {
		leaderHTTP, err := s.GetLeaderHTTPAddr()
		if err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": fmt.Sprintf("failed to get leader address: %v", err)})
		}

		// Clone the request and update the URL
		urlStr := fmt.Sprintf("http://%s/%s/%s", leaderHTTP, store, key)
		req := c.Request().Clone(c.Request().Context())
		req.URL, err = url.Parse(urlStr)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to parse leader URL: %v", err)})
		}
		req.RequestURI = "" // Clear RequestURI as it's not allowed in client requests

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to forward request: %v", err)})
		}
		defer resp.Body.Close()

		// Copy the response headers and status code
		for k, v := range resp.Header {
			c.Response().Header()[k] = v
		}
		c.Response().WriteHeader(resp.StatusCode)

		// Stream the response body
		_, err = io.Copy(c.Response().Writer, resp.Body)
		if err != nil {
			return err
		}
		return nil
	}

	// Parse request body (only if we're the leader)
	var flagData struct {
		Value      interface{} `json:"value"`
		Expression string      `json:"expression,omitempty"`
	}
	if err := c.Bind(&flagData); err != nil {
		logging.Error("failed to parse request body", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Validate expression if present
	if flagData.Expression != "" {
		if _, issues := s.celEnv.Compile(flagData.Expression); issues != nil && issues.Err() != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid expression"})
		}
		// Clear any cached program for this key
		s.celCache.Delete(fmt.Sprintf("%s/%s", store, key))
	}

	// Create command
	cmd := struct {
		Type  string `json:"type"`
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}{
		Type: "flag",
		Key:  fmt.Sprintf("%s/%s", store, key),
	}

	// Convert flagData to JSON for storage
	value, err := json.Marshal(flagData)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to marshal data"})
	}
	cmd.Value = value

	// Convert command to JSON
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to marshal command"})
	}

	logging.Info("storing flag data via Raft")

	// Apply via Raft
	future := s.raft.Apply(cmdBytes, 5*time.Second)
	if err := future.Error(); err != nil {
		logging.Error("failed to apply change", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to apply change"})
	}
	logging.Info("applied flag data PUT")
	return c.JSON(http.StatusOK, flagData)
}

// handleJoin handles cluster join requests
func (s *Server) handleJoin(c echo.Context) error {
	var joinRequest struct {
		NodeID            string `json:"node_id"`
		RaftAddr          string `json:"raft_addr"`
		HTTPAddr          string `json:"http_addr"`
		RaftAdvertiseAddr string `json:"raft_advertise_addr,omitempty"`
	}

	if err := c.Bind(&joinRequest); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to parse join request"})
	}

	logging.Info("received join request", "node", joinRequest.NodeID)

	if s.raft.State() != raft.Leader {
		// Forward to leader if we're not the leader
		leaderHTTP, err := s.GetLeaderHTTPAddr()
		if err != nil {
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": fmt.Sprintf("failed to get leader address: %v", err)})
		}

		url := fmt.Sprintf("http://%s/join", leaderHTTP)
		logging.Info("forwarding join request to leader", "url", url)

		// Forward the original request body
		reqBody, err := json.Marshal(joinRequest)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to marshal join request: %v", err)})
		}

		req, err := http.NewRequest("POST", url, bytes.NewReader(reqBody))
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to create request: %v", err)})
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to forward request: %v", err)})
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return c.JSON(resp.StatusCode, map[string]string{"error": string(body)})
		}
		return c.JSON(http.StatusOK, map[string]string{"status": "forwarded"})
	}

	// Use advertised address if provided, otherwise use the Raft address
	raftAddr := joinRequest.RaftAdvertiseAddr
	if raftAddr == "" {
		raftAddr = joinRequest.RaftAddr
	}

	logging.Info("adding voter to cluster", "node", joinRequest.NodeID, "address", raftAddr)
	// Add voter configuration
	future := s.raft.AddVoter(raft.ServerID(joinRequest.NodeID), raft.ServerAddress(raftAddr), 0, 0)
	if err := future.Error(); err != nil {
		logging.Error("failed to add voter", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to add voter: %v", err)})
	}

	// Store membership information
	entry := MembershipEntry{
		NodeID:      joinRequest.NodeID,
		RaftAddr:    raftAddr,
		HTTPAddr:    joinRequest.HTTPAddr,
		LastUpdated: time.Now().Unix(),
	}

	value, err := json.Marshal(entry)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to marshal membership entry: %v", err)})
	}

	// Create command
	cmd := struct {
		Type  string `json:"type"`
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}{
		Type:  "membership",
		Key:   joinRequest.NodeID,
		Value: value,
	}

	// Convert to JSON
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to marshal command: %v", err)})
	}

	logging.Debug("storing membership data", "data", string(cmdBytes))
	// Apply via Raft
	future = s.raft.Apply(cmdBytes, 5*time.Second)
	if err := future.Error(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to apply membership update: %v", err)})
	}

	logging.Info("successfully added voter to cluster", "node", joinRequest.NodeID)
	return c.JSON(http.StatusOK, map[string]string{"status": "joined"})
}

// preWarmCache loads all flags and pre-compiles their CEL expressions
func (s *Server) preWarmCache() error {
	return s.store.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			var flagData struct {
				Value      interface{} `json:"value"`
				Expression string      `json:"expression,omitempty"`
			}
			if err := json.Unmarshal(value, &flagData); err != nil {
				continue // Skip invalid entries
			}

			// Skip if no expression
			if flagData.Expression == "" {
				continue
			}

			// Compile and cache the program
			ast, issues := s.celEnv.Compile(flagData.Expression)
			if issues != nil && issues.Err() != nil {
				continue // Skip invalid expressions
			}

			program, err := s.celEnv.Program(ast)
			if err != nil {
				continue // Skip programs that fail to compile
			}

			// Store in cache
			s.celCache.Store(string(item.Key()), program)
		}

		return nil
	})
}

// setupRaft initializes the Raft instance
func (s *Server) setupRaft() error {
	// Setup Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(s.config.NodeID)
	config.Logger = logging.NewRaftLogger(logging.Logger)

	// Setup Raft communication
	addr, err := net.ResolveTCPAddr("tcp", s.config.RaftAddr)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %v", err)
	}

	// Use advertised address if provided, otherwise use the bind address
	advertiseAddr := addr
	if s.config.RaftAdvertiseAddr != "" {
		advertiseAddr, err = net.ResolveTCPAddr("tcp", s.config.RaftAdvertiseAddr)
		if err != nil {
			return fmt.Errorf("failed to resolve advertised TCP address: %v", err)
		}
	}

	transport, err := raft.NewTCPTransport(s.config.RaftAddr, advertiseAddr, 3, 10*time.Second, nil)
	if err != nil {
		return fmt.Errorf("failed to create TCP transport: %v", err)
	}

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(s.config.RaftDir, 2, nil)
	if err != nil {
		return fmt.Errorf("failed to create snapshot store: %v", err)
	}

	// Create the log store and stable store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(s.config.RaftDir, "raft.db"))
	if err != nil {
		return fmt.Errorf("failed to create bolt store: %v", err)
	}

	// Instantiate the Raft system
	ra, err := raft.NewRaft(config, (*fsm)(s), logStore, logStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("failed to create raft instance: %v", err)
	}
	s.raft = ra

	// Bootstrap if specified
	if s.config.Bootstrap {
		// Create a configuration with this node as the only voter
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: raft.ServerAddress(advertiseAddr.String()),
				},
			},
		}
		// Bootstrap the cluster
		future := ra.BootstrapCluster(configuration)
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to bootstrap cluster: %v", err)
		}

		// Wait for this node to become leader
		timeout := time.After(5 * time.Second)
		for {
			if ra.State() == raft.Leader {
				break
			}
			select {
			case <-timeout:
				return fmt.Errorf("timeout waiting for node to become leader")
			case <-time.After(100 * time.Millisecond):
			}
		}

		// Store bootstrap node's membership information via Raft
		entry := MembershipEntry{
			NodeID:      s.config.NodeID,
			RaftAddr:    s.config.RaftAddr,
			HTTPAddr:    s.config.HTTPAddr,
			LastUpdated: time.Now().Unix(),
		}

		value, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("failed to marshal bootstrap node membership: %v", err)
		}

		// Create command
		cmd := struct {
			Type  string `json:"type"`
			Key   string `json:"key"`
			Value []byte `json:"value"`
		}{
			Type:  "membership",
			Key:   s.config.NodeID,
			Value: value,
		}

		cmdBytes, err := json.Marshal(cmd)
		if err != nil {
			return fmt.Errorf("failed to marshal command: %v", err)
		}

		// Apply via Raft
		future = ra.Apply(cmdBytes, 5*time.Second)
		if err := future.Error(); err != nil {
			return fmt.Errorf("failed to apply membership update: %v", err)
		}

		logging.Info("successfully bootstrapped cluster", "node", config.LocalID, "address", advertiseAddr.String())
	} else {
		// For non-bootstrap nodes, wait a bit for the bootstrap node to become leader
		time.Sleep(2 * time.Second)
	}

	// Pre-warm CEL cache if enabled
	if s.config.PreWarm {
		if err := s.preWarmCache(); err != nil {
			logging.Warn("failed to pre-warm cache", "error", err)
		}
	}

	return nil
}

// handleList returns all flags in a store
func (s *Server) handleList(c echo.Context) error {
	store := c.Param("store")
	if err := validateStoreName(store); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	flags := make(map[string]interface{})
	err := s.store.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte(store + "/")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			key := string(item.Key())
			err := item.Value(func(val []byte) error {
				var value interface{}
				if err := json.Unmarshal(val, &value); err != nil {
					return err
				}
				flags[key] = value
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to list flags: %v", err))
	}

	return c.JSON(http.StatusOK, flags)
}

// handleListStores returns all available stores
func (s *Server) handleListStores(c echo.Context) error {
	stores := make(map[string]bool)
	err := s.store.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := string(item.Key())
			// Extract store name from key (format: "store/key")
			if parts := strings.Split(key, "/"); len(parts) > 1 {
				store := parts[0]
				// Skip private stores (starting with __)
				if !strings.HasPrefix(store, "__") {
					stores[store] = true
				}
			}
		}
		return nil
	})

	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to list stores: %v", err))
	}

	// Convert map to slice
	storeList := make([]string, 0, len(stores))
	for store := range stores {
		storeList = append(storeList, store)
	}

	return c.JSON(http.StatusOK, storeList)
}
