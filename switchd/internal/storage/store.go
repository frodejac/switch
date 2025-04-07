package storage

import (
	"context"
	"errors"
)

// ErrKeyNotFound is returned when a key is not found in the store
var ErrKeyNotFound = errors.New("key not found")

// Store defines the interface for storage operations
type Store interface {
	// Close closes the store and releases any resources
	Close() error

	// Get retrieves a value for a given key
	Get(ctx context.Context, key string) ([]byte, error)

	// Put stores a value for a given key
	Put(ctx context.Context, key string, value []byte) error

	// Delete removes a value for a given key
	Delete(ctx context.Context, key string) error

	// List returns all keys with a given prefix
	List(ctx context.Context, prefix string) ([]string, error)

	// ListWithValues returns all key-value pairs with a given prefix
	ListWithValues(ctx context.Context, prefix string) (map[string][]byte, error)

	// CreateSnapshot creates a snapshot of the store
	CreateSnapshot() (any, error)

	// RestoreSnapshot restores the store from a snapshot
	RestoreSnapshot(any) error
}

// Snapshot represents a point-in-time snapshot of the store
type Snapshot struct {
	path string
	db   any // This will be *badger.DB in the BadgerDB implementation
}

// MembershipEntry represents a node's metadata in the membership register
type MembershipEntry struct {
	NodeID      string `json:"node_id"`
	RaftAddr    string `json:"raft_addr"`
	HTTPAddr    string `json:"http_addr"`
	LastUpdated int64  `json:"last_updated"`
}

// MembershipStore extends Store with membership-specific operations
type MembershipStore interface {
	Store

	// GetMembership retrieves membership information for a node
	GetMembership(ctx context.Context, nodeID string) (*MembershipEntry, error)

	// PutMembership stores membership information for a node
	PutMembership(ctx context.Context, entry *MembershipEntry) error

	// DeleteMembership removes membership information for a node
	DeleteMembership(ctx context.Context, nodeID string) error

	// ListMemberships returns all membership entries
	ListMemberships(ctx context.Context) ([]*MembershipEntry, error)
}

type FeatureFlagType string

const (
	FeatureFlagTypeBoolean FeatureFlagType = "boolean"
	FeatureFlagTypeString                  = "string"
	FeatureFlagTypeInt                     = "int"
	FeatureFlagTypeFloat                   = "float"
	FeatureFlagTypeJSON                    = "json"
	FeatureFlagTypeCEL                     = "cel"
)

type FeatureFlagEntry struct {
	Type  FeatureFlagType `json:"type"`
	Value any             `json:"value"`
}

type FeatureFlagStore interface {
	Store

	// GetFeatureFlag retrieves a feature flag by name
	GetFeatureFlag(ctx context.Context, store, key string) (*FeatureFlagEntry, error)
	// PutFeatureFlag stores a feature flag
	PutFeatureFlag(ctx context.Context, store, key string, entry *FeatureFlagEntry) error
	// DeleteFeatureFlag removes a feature flag
	DeleteFeatureFlag(ctx context.Context, store, key string) error
	// ListFeatureFlags returns all feature flags in a given store
	ListFeatureFlags(ctx context.Context, store string) ([]*FeatureFlagEntry, error)
	// ListFeatureFlagsWithValues returns all feature flags with their values in a given store
	ListFeatureFlagsWithValues(ctx context.Context, store string) (map[string]*FeatureFlagEntry, error)
	// ListStores returns all stores
	ListStores(ctx context.Context) ([]string, error)
}
