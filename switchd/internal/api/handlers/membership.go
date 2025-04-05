package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/frodejac/switch/switchd/internal/consensus"
	"github.com/frodejac/switch/switchd/internal/logging"
	"github.com/frodejac/switch/switchd/internal/storage"
	"github.com/hashicorp/raft"
	"github.com/labstack/echo/v4"
)

type MembershipHandler struct {
	raft       *consensus.RaftNode
	membership storage.MembershipStore
}

func NewMembershipHandler(raft *consensus.RaftNode, membership storage.MembershipStore) *MembershipHandler {
	return &MembershipHandler{
		raft:       raft,
		membership: membership,
	}
}

func (h *MembershipHandler) HandleJoin(c echo.Context) error {
	var joinRequest struct {
		NodeID            string `json:"node_id"`
		RaftAddr          string `json:"raft_addr"`
		HTTPAddr          string `json:"http_addr"`
		RaftAdvertiseAddr string `json:"raft_advertise_addr,omitempty"`
	}

	if err := c.Bind(&joinRequest); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "failed to parse join request"})
	}

	logging.Info("received join request", "node", joinRequest.NodeID, "raft_addr", joinRequest.RaftAddr, "http_addr", joinRequest.HTTPAddr, "advertise_addr", joinRequest.RaftAdvertiseAddr)

	if h.raft.GetState() != raft.Leader {
		// Forward to leader if we're not the leader
		leaderHTTP, err := h.getLeaderHTTPAddr()
		if err != nil {
			logging.Error("failed to get leader address", "error", err)
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": fmt.Sprintf("failed to get leader address: %v", err)})
		}

		urlStr := fmt.Sprintf("http://%s/join", leaderHTTP)
		logging.Info("forwarding join request to leader", "urlStr", urlStr)

		// Forward the original request body
		reqBody, err := json.Marshal(joinRequest)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to marshal join request: %v", err)})
		}

		req, err := http.NewRequest("POST", urlStr, bytes.NewReader(reqBody))
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

	// Store membership information
	entry := &storage.MembershipEntry{
		NodeID:      joinRequest.NodeID,
		RaftAddr:    raftAddr,
		HTTPAddr:    joinRequest.HTTPAddr,
		LastUpdated: time.Now().Unix(),
	}

	// Add the server to the configuration
	logging.Info("adding voter to cluster", "node", joinRequest.NodeID, "address", raftAddr)
	addFuture := h.raft.AddVoter(raft.ServerID(joinRequest.NodeID), raft.ServerAddress(raftAddr), 0, 0)
	if err := addFuture.Error(); err != nil {
		logging.Error("failed to add voter", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to add voter: %v", err)})
	}

	// Create command
	cmd := struct {
		Type  string `json:"type"`
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}{
		Type: "membership",
		Key:  joinRequest.NodeID,
	}

	// Marshal the membership entry
	value, err := json.Marshal(entry)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to marshal membership entry: %v", err)})
	}
	cmd.Value = value

	// Apply via Raft
	logging.Info("adding membership entry", "node", joinRequest.NodeID, "address", raftAddr)
	if err := h.raft.Apply(cmd, 5*time.Second); err != nil {
		logging.Error("failed to apply membership update", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to apply membership update: %v", err)})
	}

	logging.Info("successfully added voter to cluster", "node", joinRequest.NodeID)
	return c.JSON(http.StatusOK, map[string]string{"status": "joined"})
}

func (h *MembershipHandler) HandleStatus(c echo.Context) error {
	state := h.raft.GetState()
	return c.JSON(http.StatusOK, map[string]string{
		"state": state.String(),
	})
}

// getLeaderHTTPAddr returns the HTTP address of the current leader
func (h *MembershipHandler) getLeaderHTTPAddr() (string, error) {
	leaderId := h.raft.GetLeaderId()
	logging.Debug("got leader id", "leader_id", leaderId)
	membership, err := h.membership.GetMembership(context.Background(), leaderId)
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
