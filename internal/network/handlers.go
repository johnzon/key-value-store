package network

import (
	"encoding/json"
	"io/ioutil"
	"keyvalue-store/internal/storage"
	"net/http"
)

// HandlePut handles the "/put" endpoint
func (s *Server) HandlePut(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &payload)
	if err != nil || payload.Key == "" || payload.Value == "" {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	err = s.store.Put(payload.Key, []byte(payload.Value))
	if err != nil {
		http.Error(w, "Failed to store key-value pair", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleRead handles the "/read" endpoint
func (s *Server) HandleRead(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	value, err := s.store.Read(key)
	if err == storage.ErrKeyNotFound {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Failed to read key", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(value)
}

// HandleDelete handles the "/delete" endpoint
func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	err := s.store.Delete(key)
	if err == storage.ErrKeyNotFound {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, "Failed to delete key", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleBatchPut handles the "/batchput" endpoint
func (s *Server) HandleBatchPut(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Keys   []string `json:"keys"`
		Values []string `json:"values"`
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	err = json.Unmarshal(body, &payload)
	if err != nil || len(payload.Keys) != len(payload.Values) {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	err = s.store.BatchPut(payload.Keys, convertToByteSlices(payload.Values))
	if err != nil {
		http.Error(w, "Failed to store batch key-value pairs", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// HandleReadRange handles the "/range" endpoint
func (s *Server) HandleReadRange(w http.ResponseWriter, r *http.Request) {
	startKey := r.URL.Query().Get("start")
	endKey := r.URL.Query().Get("end")
	if startKey == "" || endKey == "" {
		http.Error(w, "Start and end keys are required", http.StatusBadRequest)
		return
	}

	results, err := s.store.ReadKeyRange(startKey, endKey)
	if err != nil {
		http.Error(w, "Failed to retrieve key range", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// Utility to convert []string to [][]byte
func convertToByteSlices(values []string) [][]byte {
	byteSlices := make([][]byte, len(values))
	for i, v := range values {
		byteSlices[i] = []byte(v)
	}
	return byteSlices
}
