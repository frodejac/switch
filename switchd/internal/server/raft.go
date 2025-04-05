package server

import (
	"fmt"
	"time"

	"github.com/frodejac/switch/switchd/internal/consensus"
	"github.com/frodejac/switch/switchd/internal/logging"
	"github.com/hashicorp/raft"
)

// setupRaft initializes the Raft instance
func (s *Server) setupRaft() error {
	// Create a new Raft node
	raftNode, err := consensus.NewRaftNode(s.config, s.store, s.membership)
	if err != nil {
		return fmt.Errorf("failed to create raft node: %v", err)
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
				return fmt.Errorf("timeout waiting for node to become leader")
			case <-time.After(100 * time.Millisecond):
			}
		}
	}

	return nil
}

// JoinCluster attempts to join an existing Raft cluster
func (s *Server) JoinCluster(addr string) error {
	logging.Info("attempting to join cluster", "address", addr)

	// Send join request to the specified address
	if err := s.raft.Join(addr); err != nil {
		return fmt.Errorf("failed to join cluster: %v", err)
	}
	return nil
}

// GetLeaderHTTPAddr returns the HTTP address of the current leader
func (s *Server) GetLeaderHTTPAddr() (string, error) {
	leaderId := s.raft.GetLeaderId()
	logging.Debug("got leader id", "leader_id", leaderId)
	membership, err := s.membership.GetMembership(nil, leaderId)
	if err != nil {
		return "", fmt.Errorf("failed to get leader membership: %v", err)
	}
	if membership == nil {
		return "", fmt.Errorf("leader membership not found")
	}
	if membership.HTTPAddr == "" {
		return "", fmt.Errorf("leader membership has no HTTP address")
	}
	logging.Info("found leader membership", "leader", leaderId, "address", membership.HTTPAddr)
	return membership.HTTPAddr, nil
}

// GetRaftState returns the current state of the Raft node
func (s *Server) GetRaftState() raft.RaftState {
	return s.raft.GetState()
}
