package server

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dgraph-io/badger/v4"
	"github.com/hashicorp/raft"

	"github.com/frodejac/switch/internal/logging"
)

// fsm implements raft.FSM interface
type fsm Server

// Apply applies a Raft log entry to the FSM
func (f *fsm) Apply(log *raft.Log) interface{} {
	logging.Debug("handling apply", "data", string(log.Data))
	var cmd struct {
		Type  string `json:"type"`
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}

	if err := json.Unmarshal(log.Data, &cmd); err != nil {
		return fmt.Errorf("failed to unmarshal command: %v", err)
	}

	switch cmd.Type {
	case "membership":
		logging.Debug("storing membership data", "data", string(log.Data))
		// Update membership register
		return f.store.Update(func(txn *badger.Txn) error {
			return txn.Set(membershipKey(cmd.Key), cmd.Value)
		})
	case "flag":
		// Handle flag updates
		logging.Debug("handling flag data", "data", string(log.Data))
		return f.store.Update(func(txn *badger.Txn) error {
			return txn.Set([]byte(cmd.Key), cmd.Value)
		})
	case "delete":
		// Handle flag deletion
		logging.Debug("deleting flag", "key", cmd.Key)
		return f.store.Update(func(txn *badger.Txn) error {
			return txn.Delete([]byte(cmd.Key))
		})
	default:
		logging.Warn("unknown command type", "type", cmd.Type)
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// Snapshot returns a snapshot of the key-value store
func (f *fsm) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{store: f.store}, nil
}

// Restore restores the key-value store from a snapshot
func (f *fsm) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	// Clear existing data
	err := f.store.DropAll()
	if err != nil {
		return fmt.Errorf("failed to clear store: %v", err)
	}

	decoder := json.NewDecoder(rc)
	for decoder.More() {
		var entry struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}
		if err := decoder.Decode(&entry); err != nil {
			return fmt.Errorf("failed to decode entry: %v", err)
		}

		// Write to BadgerDB
		err := f.store.Update(func(txn *badger.Txn) error {
			return txn.Set([]byte(entry.Key), []byte(entry.Value))
		})
		if err != nil {
			return fmt.Errorf("failed to restore entry: %v", err)
		}
	}

	return nil
}

// fsmSnapshot implements the raft.FSMSnapshot interface
type fsmSnapshot struct {
	store *badger.DB
}

// Persist writes the snapshot to the given sink
func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	defer sink.Close()

	// Stream all key-value pairs
	err := f.store.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		it := txn.NewIterator(opts)
		defer it.Close()

		encoder := json.NewEncoder(sink)
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			value, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}

			entry := struct {
				Key   string          `json:"key"`
				Value json.RawMessage `json:"value"`
			}{
				Key:   string(key),
				Value: value,
			}

			if err := encoder.Encode(entry); err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to write snapshot: %v", err)
	}

	return nil
}

// Release is a no-op
func (f *fsmSnapshot) Release() {}
