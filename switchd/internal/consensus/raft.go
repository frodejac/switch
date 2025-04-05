package consensus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"time"

	"github.com/frodejac/switch/switchd/internal/config"
	"github.com/frodejac/switch/switchd/internal/logging"
	"github.com/frodejac/switch/switchd/internal/storage"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// RaftNode represents a Raft consensus node
type RaftNode struct {
	id         string
	config     *config.ServerConfig
	store      storage.Store
	membership storage.MembershipStore
	raft       *raft.Raft
	fsm        *FSM
}

// NewRaftNode creates a new Raft node
func NewRaftNode(config *config.ServerConfig, store storage.Store, membership storage.MembershipStore) (*RaftNode, error) {
	node := &RaftNode{
		id:         config.Node.ID,
		config:     config,
		store:      store,
		membership: membership,
		fsm:        NewFSM(store, membership),
	}

	if err := node.setupRaft(); err != nil {
		return nil, fmt.Errorf("failed to setup raft: %v", err)
	}

	return node, nil
}

// setupRaft initializes the Raft instance
func (r *RaftNode) setupRaft() error {
	// Setup Raft configuration
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(r.config.Node.ID)
	config.Logger = logging.NewRaftLogger(logging.Logger)

	// Setup Raft communication
	addr, err := net.ResolveTCPAddr("tcp", r.config.Raft.Address)
	if err != nil {
		return fmt.Errorf("failed to resolve TCP address: %v", err)
	}

	// Use advertised address if provided, otherwise use the bind address
	advertiseAddr := addr
	if r.config.Raft.AdvertiseAddr != "" {
		advertiseAddr, err = net.ResolveTCPAddr("tcp", r.config.Raft.AdvertiseAddr)
		if err != nil {
			return fmt.Errorf("failed to resolve advertised TCP address: %v", err)
		}
	}

	transport, err := raft.NewTCPTransport(r.config.Raft.Address, advertiseAddr, 3, 10*time.Second, nil)
	if err != nil {
		return fmt.Errorf("failed to create TCP transport: %v", err)
	}

	// Create the snapshot store
	snapshots, err := raft.NewFileSnapshotStore(r.config.Raft.Directory, 2, nil)
	if err != nil {
		return fmt.Errorf("failed to create snapshot store: %v", err)
	}

	// Create the log store and stable store
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(r.config.Raft.Directory, "raft.db"))
	if err != nil {
		return fmt.Errorf("failed to create bolt store: %v", err)
	}

	// Instantiate the Raft system
	ra, err := raft.NewRaft(config, r.fsm, logStore, logStore, snapshots, transport)
	if err != nil {
		return fmt.Errorf("failed to create raft instance: %v", err)
	}
	r.raft = ra

	// Bootstrap if specified
	if r.config.Raft.Bootstrap {
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
			NodeID:      r.config.Node.ID,
			RaftAddr:    r.config.Raft.Address,
			HTTPAddr:    r.config.HTTP.Address,
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
			Key:   r.config.Node.ID,
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

	return nil
}

// GetState returns the current state of the Raft node
func (r *RaftNode) GetState() raft.RaftState {
	return r.raft.State()
}

func (r *RaftNode) GetLeaderId() string {
	_, leaderId := r.raft.LeaderWithID()
	return string(leaderId)
}

func (r *RaftNode) GetId() string {
	return r.id
}

// Join joins an existing Raft cluster
func (r *RaftNode) Join(addr string) error {
	// Create join request with node metadata
	joinRequest := struct {
		NodeID            string `json:"node_id"`
		RaftAddr          string `json:"raft_addr"`
		HTTPAddr          string `json:"http_addr"`
		RaftAdvertiseAddr string `json:"raft_advertise_addr,omitempty"`
	}{
		NodeID:            r.config.Node.ID,
		RaftAddr:          r.config.Raft.Address,
		HTTPAddr:          r.config.HTTP.Address,
		RaftAdvertiseAddr: r.config.Raft.AdvertiseAddr,
	}

	// Marshal the join request
	reqBody, err := json.Marshal(joinRequest)
	if err != nil {
		return fmt.Errorf("failed to marshal join request: %v", err)
	}

	// Send join request to the specified address
	url := fmt.Sprintf("http://%s/join", addr)
	logging.Info("sending join request", "url", url, "node_id", r.config.Node.ID, "raft_addr", r.config.Raft.Address, "http_addr", r.config.HTTP.Address, "advertise_addr", r.config.Raft.AdvertiseAddr)

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
	timeout := time.After(r.config.Raft.JoinTimeout)
	for {
		// Check if we're part of the cluster by looking at the configuration
		future := r.raft.GetConfiguration()
		if err := future.Error(); err != nil {
			logging.Warn("failed to get raft configuration", "error", err)
			select {
			case <-timeout:
				return fmt.Errorf("timeout waiting to be added to cluster")
			case <-time.After(100 * time.Millisecond):
			}
			continue
		}

		// Check if our node is in the configuration
		config := future.Configuration()
		logging.Debug("current cluster configuration", "config", config)
		for _, server := range config.Servers {
			if server.ID == raft.ServerID(r.config.Node.ID) {
				logging.Info("successfully joined cluster")
				return nil
			}
		}

		// Check if we're in a valid state
		state := r.raft.State()
		if state == raft.Follower || state == raft.Leader {
			logging.Info("node is in a valid state", "state", state)
			return nil
		}

		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting to be added to cluster")
		case <-time.After(100 * time.Millisecond):
		}
	}
}

// Shutdown gracefully stops the Raft node
func (r *RaftNode) Shutdown() error {
	if r.raft != nil {
		r.raft.Shutdown()
	}
	return nil
}

// Apply applies a command to the Raft cluster
func (r *RaftNode) Apply(cmd any, timeout time.Duration) error {
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %v", err)
	}

	future := r.raft.Apply(cmdBytes, timeout)
	return future.Error()
}

// GetConfiguration returns the current Raft configuration
func (r *RaftNode) GetConfiguration() raft.ConfigurationFuture {
	return r.raft.GetConfiguration()
}

// AddVoter adds a voter to the Raft cluster
func (r *RaftNode) AddVoter(id raft.ServerID, addr raft.ServerAddress, prevIndex, prevTerm uint64) raft.IndexFuture {
	return r.raft.AddVoter(id, addr, prevIndex, 0)
}
