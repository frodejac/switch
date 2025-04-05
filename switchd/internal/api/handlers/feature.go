package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/frodejac/switch/switchd/internal/consensus"
	"github.com/frodejac/switch/switchd/internal/logging"
	"github.com/frodejac/switch/switchd/internal/rules"
	"github.com/frodejac/switch/switchd/internal/storage"
	"github.com/hashicorp/raft"

	"github.com/labstack/echo/v4"
	"github.com/mssola/useragent"
)

type FeatureHandler struct {
	store      storage.Store
	rules      *rules.Engine
	cache      *rules.Cache
	raft       *consensus.RaftNode
	membership storage.MembershipStore
}

func NewFeatureHandler(store storage.Store, rules *rules.Engine, cache *rules.Cache, raft *consensus.RaftNode, membership storage.MembershipStore) *FeatureHandler {
	return &FeatureHandler{
		store:      store,
		rules:      rules,
		cache:      cache,
		raft:       raft,
		membership: membership,
	}
}

func (h *FeatureHandler) HandleGet(c echo.Context) error {
	store := c.Param("store")
	if err := validateStoreName(store); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	key := c.Param("key")

	// Get the flag value
	value, err := h.store.Get(context.Background(), fmt.Sprintf("%s/%s", store, key))
	if err != nil {
		if errors.Is(err, storage.ErrKeyNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "flag not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to get flag: %v", err))
	}

	// Parse the flag data
	var flagData struct {
		Value      any    `json:"value"`
		Expression string `json:"expression,omitempty"`
	}
	if err := json.Unmarshal(value, &flagData); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to parse flag data: %v", err))
	}

	// If there's no expression, return the value directly
	if flagData.Expression == "" {
		return c.JSON(http.StatusOK, flagData.Value)
	}

	// Get the user agent and IP
	ua := useragent.New(c.Request().UserAgent())
	ip := getClientIP(c.Request())

	// Create context for evaluation
	ctx := &rules.Context{
		Key:     key,
		Context: map[string]any{},
		Request: map[string]any{
			"ip": ip,
		},
		Device: map[string]any{
			"mobile": ua.Mobile(),
			"bot":    ua.Bot(),
			"browser": func() map[string]any {
				name, version := ua.Browser()
				return map[string]any{
					"name":    name,
					"version": version,
				}
			}(),
			"os": map[string]any{
				"name": ua.OS(),
			},
		},
		Time: time.Now(),
	}

	// Add query parameters to context
	for k, v := range c.QueryParams() {
		if len(v) > 0 {
			ctx.Context[k] = v[0]
		}
	}

	// Get or compile the program
	program, ok := h.cache.Get(fmt.Sprintf("%s/%s", store, key))
	if !ok {
		var err error
		program, err = h.rules.Compile(flagData.Expression)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to compile expression: %v", err))
		}
		h.cache.Set(fmt.Sprintf("%s/%s", store, key), program)
	}

	// Evaluate the expression
	result, err := h.rules.Evaluate(program, ctx)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to evaluate expression: %v", err))
	}

	return c.JSON(http.StatusOK, result)
}

func (h *FeatureHandler) HandlePut(c echo.Context) error {
	store := c.Param("store")
	if err := validateStoreName(store); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	key := c.Param("key")

	// If not leader, forward to leader immediately
	if h.raft.GetState() != raft.Leader {
		leaderHTTP, err := h.getLeaderHTTPAddr()
		if err != nil {
			logging.Error("failed to get leader address", "error", err)
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
		Value      any    `json:"value"`
		Expression string `json:"expression,omitempty"`
	}
	if err := c.Bind(&flagData); err != nil {
		logging.Error("failed to parse request body", "error", err)
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	// Convert flagData to JSON for storage
	value, err := json.Marshal(flagData)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to marshal data"})
	}

	// Create command
	cmd := struct {
		Type  string `json:"type"`
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}{
		Type:  "put",
		Key:   fmt.Sprintf("%s/%s", store, key),
		Value: value,
	}

	// Apply via Raft
	logging.Debug("storing flag via Raft", "key", cmd.Key, "value", string(cmd.Value))
	if err := h.raft.Apply(cmd, 5*time.Second); err != nil {
		logging.Error("failed to apply put", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to apply put"})
	}
	logging.Info("successfully added flag via Put", "key", key)
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (h *FeatureHandler) HandleDelete(c echo.Context) error {
	store := c.Param("store")
	if err := validateStoreName(store); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}
	key := c.Param("key")

	// If not leader, forward to leader immediately
	if h.raft.GetState() != raft.Leader {
		leaderHTTP, err := h.getLeaderHTTPAddr()
		if err != nil {
			logging.Error("failed to get leader address", "error", err)
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

	logging.Info("deleting flag via Raft")

	// Apply via Raft
	if err := h.raft.Apply(cmd, 5*time.Second); err != nil {
		logging.Error("failed to apply delete", "error", err)
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to apply delete"})
	}
	logging.Info("applied flag deletion")
	return c.NoContent(http.StatusNoContent)
}

func (h *FeatureHandler) HandleList(c echo.Context) error {
	store := c.Param("store")
	if err := validateStoreName(store); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	values, err := h.store.ListWithValues(context.Background(), store+"/")
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("failed to list flags: %v", err))
	}

	flags := make(map[string]any)
	for key, value := range values {
		var flagValue any
		if err := json.Unmarshal(value, &flagValue); err != nil {
			continue // Skip invalid entries
		}
		flags[key] = flagValue
	}

	return c.JSON(http.StatusOK, flags)
}

func (h *FeatureHandler) HandleListStores(c echo.Context) error {
	keys, err := h.store.List(context.Background(), "")
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

// validateStoreName validates a store name
func validateStoreName(store string) error {
	if store == "" {
		return fmt.Errorf("store name cannot be empty")
	}
	if strings.HasPrefix(store, "__") {
		return fmt.Errorf("store name cannot start with __")
	}
	return nil
}

// getClientIP returns the client's IP address
func getClientIP(r *http.Request) string {
	// Try X-Forwarded-For header first
	if ip := r.Header.Get("X-Forwarded-For"); ip != "" {
		return ip
	}
	// Try X-Real-IP header next
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}
	// Fall back to RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

// GetLeaderHTTPAddr returns the HTTP address of the current leader
func (h *FeatureHandler) getLeaderHTTPAddr() (string, error) {
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
