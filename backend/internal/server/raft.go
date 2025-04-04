package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/frodejac/switch/internal/logging"
	"github.com/frodejac/switch/internal/storage"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// setupRaft initializes the Raft instance
func (s *Server) setupRaft() error {
	// Setup Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(s.config.Node.ID)
	config.Logger = logging.NewRaftLogger(logging.Logger)

	// Setup Raft communication
	addr, err := net.ResolveTCPAddr("tcp", s.config.Raft.Address)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %v", err)
	}

	// Use advertised address if provided, otherwise use the bind address
	advertiseAddr := addr
	if s.config.Raft.AdvertiseAddr != "" {
		advertiseAddr, err = net.ResolveTCPAddr("tcp", s.config.Raft.AdvertiseAddr)
		if err != nil {
			return fmt.Errorf("failed to resolve advertised TCP address: %v", err)
		}
	}

	transport, err := raft.NewTCPTransport(s.config.Raft.Address, advertiseAddr, 3, 10*time.Second, nil)
	if err != nil {
		return fmt.Errorf("failed to create TCP transport: %v", err)
	}

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(s.config.Raft.Directory, 2, nil)
	if err != nil {
		return fmt.Errorf("failed to create snapshot store: %v", err)
	}

	// Create the log store and stable store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(s.config.Raft.Directory, "raft.db"))
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
	if s.config.Raft.Bootstrap {
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
		entry := &storage.MembershipEntry{
			NodeID:      s.config.Node.ID,
			RaftAddr:    s.config.Raft.Address,
			HTTPAddr:    s.config.HTTP.Address,
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
			Key:   s.config.Node.ID,
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
	if s.config.Features.PreWarmRules {
		if err := s.preWarmCache(); err != nil {
			logging.Warn("failed to pre-warm cache", "error", err)
		}
	}

	return nil
}

// GetRaftState returns the current state of the Raft node
func (s *Server) GetRaftState() raft.RaftState {
	return s.raft.State()
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
		NodeID:            s.config.Node.ID,
		RaftAddr:          s.config.Raft.Address,
		HTTPAddr:          s.config.HTTP.Address,
		RaftAdvertiseAddr: s.config.Raft.AdvertiseAddr,
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
	timeout := time.After(s.config.Raft.JoinTimeout)
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
			if server.ID == raft.ServerID(s.config.Node.ID) {
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
