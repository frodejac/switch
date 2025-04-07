package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dgraph-io/badger/v4"
	"strings"
)

// BadgerFeatureFlagStore implements the FeatureFlagStore interface using BadgerDB
type BadgerFeatureFlagStore struct {
	*BadgerStore
}

// NewBadgerFeatureFlagStore creates a new BadgerDB feature flag store
func NewBadgerFeatureFlagStore(dataDir string) (*BadgerFeatureFlagStore, error) {
	store, err := NewBadgerStore(dataDir)
	if err != nil {
		return nil, err
	}
	return &BadgerFeatureFlagStore{BadgerStore: store}, nil
}

// featureFlagKey returns the BadgerDB key for a feature flag
func featureFlagKey(store, key string) []byte {
	return []byte(fmt.Sprintf("%s/%s", store, key))
}

// GetFeatureFlag implements FeatureFlagStore.GetFeatureFlag
func (s *BadgerFeatureFlagStore) GetFeatureFlag(ctx context.Context, store, key string) (*FeatureFlagEntry, error) {
	var entry FeatureFlagEntry
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(featureFlagKey(store, key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &entry)
		})
	})
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// PutFeatureFlag implements FeatureFlagStore.PutFeatureFlag
func (s *BadgerFeatureFlagStore) PutFeatureFlag(ctx context.Context, store, key string, entry *FeatureFlagEntry) error {
	value, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(featureFlagKey(store, key), value)
	})
}

// DeleteFeatureFlag implements FeatureFlagStore.DeleteFeatureFlag
func (s *BadgerFeatureFlagStore) DeleteFeatureFlag(ctx context.Context, store, key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(featureFlagKey(store, key))
	})
}

// ListFeatureFlags implements FeatureFlagStore.ListFeatureFlags
func (s *BadgerFeatureFlagStore) ListFeatureFlags(ctx context.Context, store string) ([]*FeatureFlagEntry, error) {
	var entries []*FeatureFlagEntry
	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(store + "/")
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var entry FeatureFlagEntry
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &entry)
			})
			if err != nil {
				return err
			}
			entries = append(entries, &entry)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// ListFeatureFlagsWithValues implements FeatureFlagStore.ListFeatureFlagsWithValues
func (s *BadgerFeatureFlagStore) ListFeatureFlagsWithValues(ctx context.Context, store string) (map[string]*FeatureFlagEntry, error) {
	entries := make(map[string]*FeatureFlagEntry)
	err := s.db.View(func(txn *badger.Txn) error {
		prefix := []byte(store + "/")
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var entry FeatureFlagEntry
			err := item.Value(func(val []byte) error {
				return json.Unmarshal(val, &entry)
			})
			if err != nil {
				return err
			}
			entries[strings.TrimPrefix(string(item.Key()), string(prefix))] = &entry
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// ListStores implements FeatureFlagStore.ListStores
func (s *BadgerFeatureFlagStore) ListStores(ctx context.Context) ([]string, error) {
	// TODO: Instead of iterating over all keys, we should maintain a separate list of stores
	stores := make(map[string]struct{})
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			key := string(it.Item().Key())
			if parts := strings.Split(key, "/"); len(parts) > 1 {
				store := parts[0]
				stores[store] = struct{}{}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	var storeList []string
	for store := range stores {
		storeList = append(storeList, store)
	}
	return storeList, nil
}
