package config

import (
	"testing"
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
				Node: NodeConfig{
					ID: "node1",
				},
				HTTP: HTTPConfig{
					Address: ":8080",
				},
				Raft: RaftConfig{
					Address:   ":8081",
					Directory: "/tmp/raft",
				},
			},
			wantErr: false,
		},
		{
			name: "missing node ID",
			config: &ServerConfig{
				HTTP: HTTPConfig{
					Address: ":8080",
				},
				Raft: RaftConfig{
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
