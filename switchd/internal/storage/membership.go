package storage

import (
	"context"
	"encoding/json"
	"github.com/dgraph-io/badger/v4"
)

// BadgerMembershipStore implements the MembershipStore interface using BadgerDB
type BadgerMembershipStore struct {
	*BadgerStore
}

// NewBadgerMembershipStore creates a new BadgerDB membership store
func NewBadgerMembershipStore(dataDir string) (*BadgerMembershipStore, error) {
	store, err := NewBadgerStore(dataDir)
	if err != nil {
		return nil, err
	}
	return &BadgerMembershipStore{BadgerStore: store}, nil
}

// membershipKey returns the BadgerDB key for a node's membership entry
func membershipKey(nodeID string) []byte {
	return []byte(nodeID)
}

// GetMembership implements MembershipStore.GetMembership
func (s *BadgerMembershipStore) GetMembership(ctx context.Context, nodeID string) (*MembershipEntry, error) {
	var entry MembershipEntry
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get(membershipKey(nodeID))
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

// PutMembership implements MembershipStore.PutMembership
func (s *BadgerMembershipStore) PutMembership(ctx context.Context, entry *MembershipEntry) error {
	value, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set(membershipKey(entry.NodeID), value)
	})
}

// DeleteMembership implements MembershipStore.DeleteMembership
func (s *BadgerMembershipStore) DeleteMembership(ctx context.Context, nodeID string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete(membershipKey(nodeID))
	})
}

// ListMemberships implements MembershipStore.ListMemberships
func (s *BadgerMembershipStore) ListMemberships(ctx context.Context) ([]*MembershipEntry, error) {
	var entries []*MembershipEntry
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		prefix := []byte("__membership/")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			var entry MembershipEntry
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
