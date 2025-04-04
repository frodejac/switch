package server

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/raft"

	"github.com/frodejac/switch/internal/logging"
	"github.com/frodejac/switch/internal/storage"
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
		entry := &storage.MembershipEntry{}
		if err := json.Unmarshal(cmd.Value, entry); err != nil {
			return fmt.Errorf("failed to unmarshal membership entry: %v", err)
		}
		return f.membership.PutMembership(nil, entry)
	case "flag":
		// Handle flag updates
		logging.Debug("handling flag data", "data", string(log.Data))
		return f.store.Put(nil, cmd.Key, cmd.Value)
	case "delete":
		// Handle flag deletion
		logging.Debug("deleting flag", "key", cmd.Key)
		return f.store.Delete(nil, cmd.Key)
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

	// Create a temporary snapshot
	snapshot, err := f.store.CreateSnapshot()
	if err != nil {
		return fmt.Errorf("failed to create temporary snapshot: %v", err)
	}

	// Restore from the snapshot
	if err := f.store.RestoreSnapshot(snapshot); err != nil {
		return fmt.Errorf("failed to restore snapshot: %v", err)
	}

	return nil
}

// fsmSnapshot implements the raft.FSMSnapshot interface
type fsmSnapshot struct {
	store storage.Store
}

// Persist writes the snapshot to the given sink
func (f *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	defer sink.Close()

	// Get all key-value pairs from the snapshot
	values, err := f.store.ListWithValues(nil, "")
	if err != nil {
		sink.Cancel()
		return fmt.Errorf("failed to get values from snapshot: %v", err)
	}

	// Write all key-value pairs to the sink
	encoder := json.NewEncoder(sink)
	for key, value := range values {
		entry := struct {
			Key   string          `json:"key"`
			Value json.RawMessage `json:"value"`
		}{
			Key:   key,
			Value: value,
		}

		if err := encoder.Encode(entry); err != nil {
			sink.Cancel()
			return fmt.Errorf("failed to encode entry: %v", err)
		}
	}

	return nil
}

// Release is a no-op
func (f *fsmSnapshot) Release() {}
