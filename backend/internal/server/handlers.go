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

	"github.com/dgraph-io/badger/v4"
	"github.com/frodejac/switch/internal/logging"
	"github.com/google/cel-go/cel"
	"github.com/hashicorp/raft"
	"github.com/labstack/echo/v4"
	"github.com/mssola/useragent"
	"google.golang.org/protobuf/types/known/structpb"
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
		"ip":      getClientIP(c),
		"headers": c.Request().Header,
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
		"request": request,
		"device":  device,
		"time":    time.Now(),
	})
	if err != nil {
		logging.Error("evaluation failed", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "evaluation failed"})
	}

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
