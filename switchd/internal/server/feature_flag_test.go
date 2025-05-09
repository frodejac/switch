package server_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/frodejac/switch/switchd/internal/config"
	"github.com/frodejac/switch/switchd/internal/logging"
	"github.com/frodejac/switch/switchd/internal/server"
	"github.com/hashicorp/raft"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFeatureFlagE2E(t *testing.T) {
	// Initialize logger
	logging.Init(slog.LevelDebug)

	// Set up environment variables for testing
	os.Setenv("NODE_ID", "test-node")
	os.Setenv("HTTP_ADDRESS", "localhost:8080")
	os.Setenv("RAFT_ADDRESS", "localhost:8081")
	os.Setenv("RAFT_DIRECTORY", t.TempDir())
	os.Setenv("RAFT_BOOTSTRAP", "true")
	os.Setenv("FEATURES_PRE_WARM_RULES", "true")

	// Load configuration
	cfg, err := config.Load()
	require.NoError(t, err)

	srv, err := server.NewServer(cfg)
	require.NoError(t, err)

	// Start the server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := srv.Start(); err != nil {
			errCh <- err
		}
	}()

	// Wait for server to start and become leader
	timeout := time.After(5 * time.Second)
	for {
		// Try to connect to the HTTP server
		_, err := http.Get(fmt.Sprintf("http://%s/stores", cfg.HTTP.Address))
		if err == nil {
			// Server is up, now wait for it to become leader
			if srv.GetRaftState() == raft.Leader {
				break
			}
		}

		select {
		case err := <-errCh:
			t.Fatalf("server failed to start: %v", err)
		case <-timeout:
			t.Fatal("timeout waiting for server to start and become leader")
		case <-time.After(100 * time.Millisecond):
		}
	}

	// Cleanup
	defer func() {
		err := srv.Stop()
		require.NoError(t, err)
	}()

	// Test 1: Create a feature flag with a CEL expression
	store := "test-store"
	key := "test-flag"
	flagData := map[string]any{
		"value":      "default",
		"expression": `context.user_role == "admin" ? "admin-value" : "user-value"`,
	}

	// Create the feature flag
	req, err := http.NewRequest(http.MethodPut,
		fmt.Sprintf("http://%s/%s/%s", cfg.HTTP.Address, store, key),
		bytes.NewReader(mustMarshal(t, flagData)),
	)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Test 2: Retrieve the feature flag with admin context
	resp, err = http.Get(fmt.Sprintf("http://%s/%s/%s?user_role=admin", cfg.HTTP.Address, store, key))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var result any
	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "admin-value", result)

	// Test 3: Retrieve the feature flag with user context
	resp, err = http.Get(fmt.Sprintf("http://%s/%s/%s?user_role=user", cfg.HTTP.Address, store, key))
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	err = json.NewDecoder(resp.Body).Decode(&result)
	require.NoError(t, err)
	assert.Equal(t, "user-value", result)
}

func mustMarshal(t *testing.T, v any) []byte {
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
