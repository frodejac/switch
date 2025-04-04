package config

import (
	"testing"
	"time"
)

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ServerConfig{
				Node: struct {
					ID string `json:"id"`
				}{ID: "node1"},
				HTTP: struct {
					Address string `json:"address"`
				}{Address: ":8080"},
				Raft: struct {
					Address       string        `json:"address"`
					AdvertiseAddr string        `json:"advertise_addr"`
					Directory     string        `json:"directory"`
					Bootstrap     bool          `json:"bootstrap"`
					JoinAddress   string        `json:"join_address"`
					JoinTimeout   time.Duration `json:"join_timeout"`
				}{
					Address:   ":8081",
					Directory: "/tmp/raft",
				},
			},
			wantErr: false,
		},
		{
			name: "missing node ID",
			config: &ServerConfig{
				HTTP: struct {
					Address string `json:"address"`
				}{Address: ":8080"},
				Raft: struct {
					Address       string        `json:"address"`
					AdvertiseAddr string        `json:"advertise_addr"`
					Directory     string        `json:"directory"`
					Bootstrap     bool          `json:"bootstrap"`
					JoinAddress   string        `json:"join_address"`
					JoinTimeout   time.Duration `json:"join_timeout"`
				}{
					Address:   ":8081",
					Directory: "/tmp/raft",
				},
			},
			wantErr: true,
		},
		// Add more test cases
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuilder(t *testing.T) {
	config, err := NewBuilder().
		WithNodeID("node1").
		WithHTTPAddress(":8080").
		WithRaftAddress(":8081").
		WithRaftDirectory("/tmp/raft").
		WithBootstrap(true).
		Build()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if config.Node.ID != "node1" {
		t.Errorf("expected node ID 'node1', got %s", config.Node.ID)
	}
	// Add more assertions
}
