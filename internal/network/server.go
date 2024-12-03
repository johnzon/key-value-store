package network

import (
	"encoding/json"
	"io/ioutil"
	"keyvalue-store/internal/storage"
	"log"
	"net/http"
)

type Server struct {
	store *storage.Storage
}

func NewServer(store *storage.Storage) *Server {
	return &Server{store: store}
}

func (s *Server) RegisterRoutes() {
	http.HandleFunc("/put", s.handlePut)
	http.HandleFunc("/read", s.handleRead)
	http.HandleFunc("/range", s.handleReadRange)
	http.HandleFunc("/batchput", s.handleBatchPut)
	http.HandleFunc("/delete", s.handleDelete)
}

func (s *Server) Start(address string) {
	log.Printf("Starting server on %s...\n", address)

	// Register the routes
	s.RegisterRoutes()

	// Start listening for incoming requests
	log.Fatal(http.ListenAndServe(address, nil))
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Invalid HTTP method", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	value, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}

	err = s.store.Put(key, value)
	if err != nil {
		http.Error(w, "Failed to store key-value pair", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid HTTP method", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	value, err := s.store.Read(key)
	if err != nil {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(value)
}

func (s *Server) handleReadRange(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Invalid HTTP method", http.StatusMethodNotAllowed)
		return
	}

	startKey := r.URL.Query().Get("start")
	endKey := r.URL.Query().Get("end")
	if startKey == "" || endKey == "" {
		http.Error(w, "StartKey and EndKey are required", http.StatusBadRequest)
		return
	}

	results, err := s.store.ReadKeyRange(startKey, endKey)
	if err != nil {
		http.Error(w, "Failed to retrieve key range", http.StatusInternalServerError)
		return
	}

	jsonResponse, err := json.Marshal(results)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(jsonResponse)
}

func (s *Server) handleBatchPut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Invalid HTTP method", http.StatusMethodNotAllowed)
		return
	}

	var batch map[string][]byte
	err := json.NewDecoder(r.Body).Decode(&batch)
	if err != nil {
		http.Error(w, "Failed to decode JSON", http.StatusBadRequest)
		return
	}

	for key, value := range batch {
		if err := s.store.Put(key, value); err != nil {
			http.Error(w, "Failed to store key-value pair", http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Invalid HTTP method", http.StatusMethodNotAllowed)
		return
	}

	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	err := s.store.Delete(key)
	if err != nil {
		http.Error(w, "Failed to delete key", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
