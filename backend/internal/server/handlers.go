package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/frodejac/switch/internal/logging"
	"github.com/frodejac/switch/internal/rules"
	"github.com/frodejac/switch/internal/storage"
	"github.com/hashicorp/raft"
	"github.com/labstack/echo/v4"
	"github.com/mssola/useragent"
)

// handleDelete handles DELETE requests for feature flags
func (s *Server) handleDelete(c echo.Context) error {
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

	// Create command
	cmd := struct {
		Type  string `json:"type"`
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}{
		Type: "delete",
		Key:  fmt.Sprintf("%s/%s", store, key),
	}

	// Convert command to JSON
	cmdBytes, err := json.Marshal(cmd)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to marshal command"})
	}

	logging.Info("deleting flag via Raft")

	// Apply via Raft
	future := s.raft.Apply(cmdBytes, 5*time.Second)
	if err := future.Error(); err != nil {
		logging.Error("failed to apply delete", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to apply delete"})
	}
	logging.Info("applied flag deletion")
	return c.NoContent(http.StatusNoContent)
}

// handleListStores returns all available stores
func (s *Server) handleListStores(c echo.Context) error {
	keys, err := s.store.List(nil, "")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to list stores: %v", err))
	}

	stores := make(map[string]bool)
	for _, key := range keys {
		// Extract store name from key (format: "store/key")
		if parts := strings.Split(key, "/"); len(parts) > 1 {
			store := parts[0]
			// Skip private stores (starting with __)
			if !strings.HasPrefix(store, "__") {
				stores[store] = true
			}
		}
	}

	// Convert map to slice
	storeList := make([]string, 0, len(stores))
	for store := range stores {
		storeList = append(storeList, store)
	}

	return c.JSON(http.StatusOK, storeList)
}

// handleList returns all flags in a store
func (s *Server) handleList(c echo.Context) error {
	store := c.Param("store")
	if err := validateStoreName(store); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	values, err := s.store.ListWithValues(nil, store+"/")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to list flags: %v", err))
	}

	flags := make(map[string]interface{})
	for key, value := range values {
		var flagValue interface{}
		if err := json.Unmarshal(value, &flagValue); err != nil {
			continue // Skip invalid entries
		}
		flags[key] = flagValue
	}

	return c.JSON(http.StatusOK, flags)
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
	entry := &storage.MembershipEntry{
		NodeID:      joinRequest.NodeID,
		RaftAddr:    raftAddr,
		HTTPAddr:    joinRequest.HTTPAddr,
		LastUpdated: time.Now().Unix(),
	}

	if err := s.membership.PutMembership(nil, entry); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("failed to store membership: %v", err)})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "joined"})
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
		// Use the rules engine to validate the expression
		if _, err := s.rules.Compile(flagData.Expression); err != nil {
			if compErr, ok := err.(*rules.CompilationError); ok {
				return c.JSON(http.StatusBadRequest, map[string]interface{}{
					"error": "invalid expression",
					"details": map[string]interface{}{
						"expression": compErr.Expression,
						"issues":     compErr.Issues,
					},
				})
			}
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		// Clear any cached program for this key
		s.cache.Delete(fmt.Sprintf("%s/%s", store, key))
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

	// Create request context
	request := map[string]interface{}{
		"ip": getClientIP(c),
		"headers": func() map[string]interface{} {
			headers := make(map[string]interface{})
			for k, v := range c.Request().Header {
				if len(v) > 0 {
					headers[k] = v[0]
				}
			}
			return headers
		}(),
	}

	// Parse user agent for device info
	ua := useragent.New(c.Request().UserAgent())
	browserName, browserVersion := ua.Browser()
	device := map[string]interface{}{
		"mobile": ua.Mobile(),
		"bot":    ua.Bot(),
		"os":     ua.OS(),
		"browser": map[string]interface{}{
			"name":    browserName,
			"version": browserVersion,
		},
	}

	// Read from storage
	value, err := s.store.Get(nil, fmt.Sprintf("%s/%s", store, key))
	if err != nil {
		if err == storage.ErrKeyNotFound {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "key not found"})
		}
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
	var program *rules.Program
	if cached, ok := s.cache.Get(cacheKey); ok {
		program = cached
	} else {
		// Compile and cache the program
		var err error
		program, err = s.rules.Compile(flagData.Expression)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "invalid expression"})
		}
		s.cache.Set(cacheKey, program)
	}

	// Create evaluation context
	ctx := &rules.Context{
		Key:     key,
		Context: context,
		Request: request,
		Device:  device,
		Time:    time.Now(),
	}

	// Evaluate the rule
	result, err := s.rules.Evaluate(program, ctx)
	if err != nil {
		logging.Error("failed to evaluate expression",
			"error", err,
			"expression", flagData.Expression,
			"context", ctx,
		)

		if evalErr, ok := err.(*rules.EvaluationError); ok {
			return c.JSON(http.StatusInternalServerError, map[string]interface{}{
				"error": "failed to evaluate expression",
				"details": map[string]interface{}{
					"expression": flagData.Expression,
					"error":      evalErr.Error(),
				},
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": err.Error()})
	}

	// Return the raw result
	return c.JSON(http.StatusOK, result)
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

// getClientIP returns the real client IP address, handling X-Forwarded-For header
func getClientIP(c echo.Context) string {
	// First try X-Forwarded-For header
	forwarded := c.Request().Header.Get("X-Forwarded-For")
	if forwarded != "" {
		// X-Forwarded-For can contain multiple IPs, the first one is the client
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Then try X-Real-IP header
	realIP := c.Request().Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to the direct connection IP
	return c.RealIP()
}
