package replication

import (
	"keyvalue-store/internal/replication"
	"keyvalue-store/internal/storage"
	"net/http"
	"os"
	"testing"
	"time"
)

func TestLeaderServer(t *testing.T) {

	candidateID := "test-leader"
	port := ":8080"
	heartbeatInterval := 2 * time.Second
	heartbeatTimeout := 5 * time.Second

	// Paths for the WAL and SnapshotManager
	walPath := "test_wal.log"
	snapshotPath := "test_snapshot"

	// Clean up test files
	defer os.Remove(walPath)
	defer os.Remove(snapshotPath)

	// Create the storage system with WAL and SnapshotManager
	store, err := storage.NewStorage("test_data.log", walPath, snapshotPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer store.Close()

	server, err := replication.NewLeaderServer(candidateID, port, heartbeatInterval, heartbeatTimeout, store)

	if err != nil {
		t.Fatalf("Failed to create leader server: %v", err)
	}

	// Start the leader's HTTP server in a separate goroutine
	go server.StartLeaderHTTPServer()

	// Test if the leader is responding to heartbeat
	time.Sleep(1 * time.Second) // Wait for heartbeat

	resp, err := http.Get("http://localhost:8080/heartbeat")
	if err != nil {
		t.Fatalf("Failed to send heartbeat request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %v", resp.StatusCode)
	}
}
