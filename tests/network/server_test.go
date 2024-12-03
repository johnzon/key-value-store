package tests

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"keyvalue-store/internal/network"
	"keyvalue-store/internal/storage"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func setupServer(t *testing.T) *httptest.Server {
	// Paths for WAL and SnapshotManager
	walPath := "test_wal.log"
	snapshotPath := "test_snapshot"

	// Temporary storage for testing with WAL and SnapshotManager
	filePath := "test_data.log"
	store, err := storage.NewStorage(filePath, walPath, snapshotPath)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create the server with the storage
	server := network.NewServer(store)

	// Cleanup after tests
	t.Cleanup(func() {
		store.Close()
		os.Remove(filePath)
		os.Remove(walPath)
		os.Remove(snapshotPath)
	})

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/put":
			server.HandlePut(w, r)
		case "/read":
			server.HandleRead(w, r)
		case "/delete":
			server.HandleDelete(w, r)
		case "/batchput":
			server.HandleBatchPut(w, r)
		case "/range":
			server.HandleReadRange(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestServerEndpoints(t *testing.T) {
	server := setupServer(t)

	// Test PUT endpoint
	t.Run("PUT key-value pair", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodPut, server.URL+"/put?key=key1", bytes.NewBuffer([]byte("value1")))
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to PUT: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test GET endpoint
	t.Run("GET key-value pair", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/read?key=key1")
		if err != nil {
			t.Fatalf("Failed to GET: %v", err)
		}
		defer resp.Body.Close()

		body, _ := ioutil.ReadAll(resp.Body)
		if string(body) != "value1" {
			t.Fatalf("Expected value 'value1', got '%s'", string(body))
		}
	})

	// Test DELETE endpoint
	t.Run("DELETE key", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodDelete, server.URL+"/delete?key=key1", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to DELETE: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test Batch PUT endpoint
	t.Run("Batch PUT key-value pairs", func(t *testing.T) {
		batch := map[string]string{"key2": "value2", "key3": "value3"}
		batchJSON, _ := json.Marshal(batch)

		req, _ := http.NewRequest(http.MethodPost, server.URL+"/batchput", bytes.NewBuffer(batchJSON))
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Failed to Batch PUT: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("Expected status 200, got %d", resp.StatusCode)
		}
	})

	// Test GET Range endpoint
	t.Run("GET key range", func(t *testing.T) {
		resp, err := http.Get(server.URL + "/range?start=key2&end=key3")
		if err != nil {
			t.Fatalf("Failed to GET range: %v", err)
		}
		defer resp.Body.Close()

		var result map[string]string
		body, _ := ioutil.ReadAll(resp.Body)
		err = json.Unmarshal(body, &result)
		if err != nil {
			t.Fatalf("Failed to decode JSON response: %v", err)
		}

		expected := map[string]string{"key2": "value2", "key3": "value3"}
		for key, value := range expected {
			if result[key] != value {
				t.Fatalf("Expected %s:%s, got %s:%s", key, value, key, result[key])
			}
		}
	})
}
