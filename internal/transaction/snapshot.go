package transaction

import (
	"encoding/gob"
	"os"
)

// Snapshot represents a snapshot of the storage system at a certain point in time
type Snapshot struct {
	Data map[string][]byte // Snapshot of key-value pairs
}

// SnapshotManager handles creating and applying snapshots
type SnapshotManager struct {
	filePath string
}

// NewSnapshotManager initializes a new snapshot manager
func NewSnapshotManager(filePath string) *SnapshotManager {
	return &SnapshotManager{filePath: filePath}
}

// CreateSnapshot saves the current state of the key-value store to a file
func (sm *SnapshotManager) CreateSnapshot(data map[string][]byte) error {
	file, err := os.OpenFile(sm.filePath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	enc := gob.NewEncoder(file)
	return enc.Encode(data)
}

// LoadSnapshot loads the snapshot data from a file
func (sm *SnapshotManager) LoadSnapshot() (map[string][]byte, error) {
	file, err := os.Open(sm.filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data map[string][]byte
	dec := gob.NewDecoder(file)
	err = dec.Decode(&data)
	return data, err
}
