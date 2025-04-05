package consensus

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/frodejac/switch/switchd/internal/logging"

	"archive/tar"

	"github.com/frodejac/switch/switchd/internal/storage"
	"github.com/hashicorp/raft"
)

// FSM represents the Raft finite state machine
type FSM struct {
	store      storage.Store
	membership storage.MembershipStore
}

// NewFSM creates a new FSM instance
func NewFSM(store storage.Store, membership storage.MembershipStore) *FSM {
	return &FSM{
		store:      store,
		membership: membership,
	}
}

// Apply applies a Raft log entry to the FSM
func (f *FSM) Apply(log *raft.Log) any {
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
		logging.Debug("applying membership command", "key", cmd.Key, "value", string(cmd.Value))
		// Update membership register
		entry := &storage.MembershipEntry{}
		if err := json.Unmarshal(cmd.Value, entry); err != nil {
			return fmt.Errorf("failed to unmarshal membership entry: %v", err)
		}
		// Store the membership entry
		if err := f.membership.PutMembership(context.Background(), entry); err != nil {
			return fmt.Errorf("failed to store membership entry: %v", err)
		}
		return nil
	case "put":
		logging.Debug("applying put command", "key", cmd.Key, "value", string(cmd.Value))
		if err := f.store.Put(context.Background(), cmd.Key, cmd.Value); err != nil {
			return fmt.Errorf("failed to store value: %v", err)
		}
		return nil
	case "delete":
		logging.Debug("applying delete command", "key", cmd.Key)
		if err := f.store.Delete(context.Background(), cmd.Key); err != nil {
			return fmt.Errorf("failed to delete key: %v", err)
		}
		return nil
	default:
		logging.Error("unknown command type", "type", cmd.Type)
		return fmt.Errorf("unknown command type: %s", cmd.Type)
	}
}

// Snapshot returns a snapshot of the FSM's state
func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &fsmSnapshot{
		store:      f.store,
		membership: f.membership,
	}, nil
}

// Restore restores an FSM from a snapshot
func (f *FSM) Restore(rc io.ReadCloser) error {
	defer rc.Close()

	// Create a new snapshot
	snap := &fsmSnapshot{
		store:      f.store,
		membership: f.membership,
	}

	// Restore the snapshot
	return snap.Restore(rc)
}

// fsmSnapshot represents a snapshot of the FSM's state
type fsmSnapshot struct {
	store      storage.Store
	membership storage.MembershipStore
}

// Persist saves the FSM snapshot to the given sink
func (s *fsmSnapshot) Persist(sink raft.SnapshotSink) error {
	// Create a snapshot of the store
	_, err := s.store.CreateSnapshot()
	if err != nil {
		return fmt.Errorf("failed to create store snapshot: %v", err)
	}

	// Create a tar writer
	tw := tar.NewWriter(sink)
	defer tw.Close()

	// Get the snapshot directory
	snapDir := filepath.Join(os.TempDir(), "badger-snapshot-*")
	matches, err := filepath.Glob(snapDir)
	if err != nil {
		return fmt.Errorf("failed to find snapshot directory: %v", err)
	}
	if len(matches) == 0 {
		return fmt.Errorf("snapshot directory not found")
	}
	snapDir = matches[0]

	// Walk through the snapshot directory and add files to the tar archive
	err = filepath.Walk(snapDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Create a tar header
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}

		// Set the name to be relative to the snapshot directory
		relPath, err := filepath.Rel(snapDir, path)
		if err != nil {
			return err
		}
		header.Name = relPath

		// Write the header
		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a file, write its contents
		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to create tar archive: %v", err)
	}

	return nil
}

// Release releases any resources associated with the snapshot
func (s *fsmSnapshot) Release() {
	// Nothing to release
}

// Restore restores the FSM state from the given reader
func (s *fsmSnapshot) Restore(rc io.ReadCloser) error {
	// Create a temporary directory for the snapshot
	tmpDir, err := os.MkdirTemp("", "raft-snapshot-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Extract the snapshot data
	if err := extractTar(rc, tmpDir); err != nil {
		return fmt.Errorf("failed to extract snapshot: %v", err)
	}

	// Restore the snapshot
	if err := s.store.RestoreSnapshot(tmpDir); err != nil {
		return fmt.Errorf("failed to restore snapshot: %v", err)
	}

	return nil
}

// extractTar extracts a tar archive to the given directory
func extractTar(rc io.ReadCloser, dest string) error {
	tr := tar.NewReader(rc)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer f.Close()
			if _, err := io.Copy(f, tr); err != nil {
				return err
			}
		}
	}
	return nil
}
