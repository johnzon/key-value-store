package tests

import (
	"keyvalue-store/internal/storage"
	"os"
	"strings"
	"testing"
)

func TestStorage(t *testing.T) {
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

	// Test Put
	err = store.Put("key1", []byte("value1"))
	if err != nil {
		t.Fatalf("Failed to put key: %v", err)
	}

	// Test BatchPut
	keys := []string{"key2", "key3"}
	values := [][]byte{[]byte("value2"), []byte("value3")}
	err = store.BatchPut(keys, values)
	if err != nil {
		t.Fatalf("Failed to batch put keys: %v", err)
	}

	// Validate BatchPut by reading back the keys
	for i, key := range keys {
		value, err := store.Read(key)
		if err != nil {
			t.Fatalf("Failed to read key '%s': %v", key, err)
		}
		value = []byte(strings.TrimSpace(string(value))) // Trim newline or spaces
		expectedValue := values[i]
		if string(value) != string(expectedValue) {
			t.Fatalf("Expected value '%s', got '%s' for key '%s'", expectedValue, value, key)
		}
	}

	// Test Read
	value, err := store.Read("key1")
	if err != nil {
		t.Fatalf("Failed to read key: %v", err)
	}
	// Trim the value to remove any extra newline or spaces
	value = []byte(strings.TrimSpace(string(value)))
	// Compare byte slices instead of string
	expectedValue := []byte("value1")
	if string(value) != string(expectedValue) {
		t.Fatalf("Expected value '%s', got '%s'", expectedValue, value)
	}

	// Test Delete
	err = store.Delete("key1")
	if err != nil {
		t.Fatalf("Failed to delete key: %v", err)
	}

	// Ensure key is deleted
	_, err = store.Read("key1")
	if err == nil {
		t.Fatalf("Expected error for deleted key, got nil")
	}
}
