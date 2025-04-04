package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/dgraph-io/badger/v4"
	"github.com/frodejac/switch/internal/logging"
)

// BadgerSnapshot represents a point-in-time snapshot of a BadgerDB store
type BadgerSnapshot struct {
	path string
	db   *badger.DB
}

// BadgerStore implements the Store interface using BadgerDB
type BadgerStore struct {
	db *badger.DB
}

// NewBadgerStore creates a new BadgerDB store
func NewBadgerStore(dataDir string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(filepath.Join(dataDir, "badger"))
	opts.Logger = logging.NewBadgerLogger(logging.Logger)

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open badger store: %v", err)
	}

	return &BadgerStore{db: db}, nil
}

// Close implements Store.Close
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// Get implements Store.Get
func (s *BadgerStore) Get(ctx context.Context, key string) ([]byte, error) {
	var value []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			value = val
			return nil
		})
	})
	if err == badger.ErrKeyNotFound {
		return nil, ErrKeyNotFound
	}
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Put implements Store.Put
func (s *BadgerStore) Put(ctx context.Context, key string, value []byte) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), value)
	})
}

// Delete implements Store.Delete
func (s *BadgerStore) Delete(ctx context.Context, key string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})
}

// List implements Store.List
func (s *BadgerStore) List(ctx context.Context, prefix string) ([]string, error) {
	var keys []string
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()
			keys = append(keys, string(item.Key()))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// ListWithValues implements Store.ListWithValues
func (s *BadgerStore) ListWithValues(ctx context.Context, prefix string) (map[string][]byte, error) {
	values := make(map[string][]byte)
	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Seek([]byte(prefix)); it.ValidForPrefix([]byte(prefix)); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				values[string(item.Key())] = val
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return values, nil
}

// CreateSnapshot implements Store.CreateSnapshot
func (s *BadgerStore) CreateSnapshot() (interface{}, error) {
	// Create a temporary directory for the snapshot
	tmpDir, err := os.MkdirTemp("", "badger-snapshot-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %v", err)
	}

	// Create a new BadgerDB instance for the snapshot
	opts := badger.DefaultOptions(filepath.Join(tmpDir, "snapshot"))
	snapshotDB, err := badger.Open(opts)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to open snapshot db: %v", err)
	}

	// Copy all data from the main DB to the snapshot DB
	err = s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				return snapshotDB.Update(func(snapTxn *badger.Txn) error {
					return snapTxn.Set(item.Key(), val)
				})
			})
			if err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		snapshotDB.Close()
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to copy data to snapshot: %v", err)
	}

	return &BadgerSnapshot{
		path: tmpDir,
		db:   snapshotDB,
	}, nil
}

// RestoreSnapshot implements Store.RestoreSnapshot
func (s *BadgerStore) RestoreSnapshot(snapshot interface{}) error {
	badgerSnapshot, ok := snapshot.(*BadgerSnapshot)
	if !ok {
		return fmt.Errorf("invalid snapshot type")
	}

	// Close the current DB
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("failed to close current db: %v", err)
	}

	// Get the current DB path
	currentPath := s.db.Opts().Dir

	// Remove the current DB files
	if err := os.RemoveAll(currentPath); err != nil {
		return fmt.Errorf("failed to remove current db: %v", err)
	}

	// Copy snapshot files to the current DB path
	if err := copyDir(badgerSnapshot.path, filepath.Dir(currentPath)); err != nil {
		return fmt.Errorf("failed to copy snapshot: %v", err)
	}

	// Reopen the DB
	opts := badger.DefaultOptions(currentPath)
	newDB, err := badger.Open(opts)
	if err != nil {
		return fmt.Errorf("failed to reopen db: %v", err)
	}

	s.db = newDB
	return nil
}

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
	return []byte(fmt.Sprintf("__membership/%s", nodeID))
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

// copyDir recursively copies a directory
func copyDir(src, dst string) error {
	// Create destination directory
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// copyFile copies a single file
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}
