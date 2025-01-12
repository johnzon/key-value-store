package network

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"keyvalue-store/internal/node"
	"keyvalue-store/internal/storage"
	"net/http"
)

func (s *Server) HandleRead(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	value, err := s.Node.HandleGet(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			http.Error(w, "Key not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to read key", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(value)
}

func (s *Server) HandleDelete(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		http.Error(w, "Key is required", http.StatusBadRequest)
		return
	}

	err := s.Node.HandleDelete(key)
	if err != nil {
		if err == storage.ErrKeyNotFound {
			http.Error(w, "Key not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete key", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) HandleWrite(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		Key   string `json:"key"`
		Value []byte `json:"value"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	err := s.Node.HandleWrite(payload.Key, payload.Value)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to write: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Write successful"))
}

func (s *Server) HandleBatchWrite(w http.ResponseWriter, r *http.Request) {
	var entries map[string][]byte
	if err := json.NewDecoder(r.Body).Decode(&entries); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	err := s.Node.HandleBatchPut(entries)
	if err != nil {
		http.Error(w, fmt.Sprintf("Batch write failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Batch write successful"))
}

func (s *Server) HandleBatchQuery(w http.ResponseWriter, r *http.Request) {
	startKey := r.URL.Query().Get("startKey")
	endKey := r.URL.Query().Get("endKey")
	if startKey == "" || endKey == "" {
		http.Error(w, "startKey and endKey are required", http.StatusBadRequest)
		return
	}

	results, err := s.Node.HandleQueryBatch(startKey, endKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("Batch query failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	var hbReq node.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&hbReq); err != nil {
		http.Error(w, "Invalid heartbeat payload", http.StatusBadRequest)
		return
	}
	s.Node.HandleHeartbeat(hbReq.Term, hbReq.NodeID, hbReq.URL)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Heartbeat acknowledged from leader %s for term %d", hbReq.NodeID, hbReq.Term)
}

func (s *Server) HandleRegisterFollower(w http.ResponseWriter, r *http.Request) {
	var hbReq node.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&hbReq); err != nil {
		http.Error(w, "Invalid heartbeat payload", http.StatusBadRequest)
		return
	}
	s.Node.RegisterFollower(hbReq.NodeID, hbReq.URL)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "follower  %s registered with url %d", hbReq.NodeID, hbReq.URL)
}

func (s *Server) handleVoteRequest(w http.ResponseWriter, r *http.Request) {
	var payload struct {
		CandidateID  string `json:"candidateID"`
		Term         int    `json:"term"`
		LastLogIndex int64  `json:"lastLogIndex"`
		LastLogTerm  int64  `json:"lastLogTerm"`
	}

	// Decode the request body into the payload struct
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Delegate the vote request processing to the Node logic
	voteGranted, currentTerm := s.Node.HandleVoteRequest(payload.CandidateID, payload.Term, payload.LastLogIndex, payload.LastLogTerm)

	// Prepare the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]any{
		"voteGranted": voteGranted,
		"term":        currentTerm,
	})
}

// ReplicateHandler handles the replication HTTP request from the leader.
func (h *Server) HandleReplication(w http.ResponseWriter, r *http.Request) {
	var replicationRequest struct {
		LeaderID  string         `json:"leaderID"`
		Term      int            `json:"term"`
		Key       string         `json:"key"`
		Value     string         `json:"value"` // Base64 encoded
		Timestamp int64          `json:"timestamp"`
		Operation node.Operation `json:"operation"`
	}

	// Decode JSON payload
	if err := json.NewDecoder(r.Body).Decode(&replicationRequest); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Decode base64 value
	value, err := base64.StdEncoding.DecodeString(replicationRequest.Value)
	if err != nil {
		http.Error(w, "Invalid base64 value", http.StatusBadRequest)
		return
	}

	// Delegate the logic to the node
	if err := h.Node.ProcessReplication(replicationRequest.LeaderID, replicationRequest.Term, replicationRequest.Key, value, replicationRequest.Timestamp, replicationRequest.Operation); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Replication successful"))
}

// BatchReplicateHandler handles batch replication requests from the leader.
func (h *Server) HandleBatchReplicate(w http.ResponseWriter, r *http.Request) {

	var request struct {
		LeaderID  string            `json:"leaderID"`
		Term      int               `json:"term"`
		Entries   map[string]string `json:"entries"` // Key-value pairs encoded as strings
		Timestamp int64             `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request payload: %v", err), http.StatusBadRequest)
		return
	}

	decodedEntries := make(map[string][]byte)
	for key, value := range request.Entries {
		decodedValue, err := base64.StdEncoding.DecodeString(value)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to decode value for key %s: %v", key, err), http.StatusBadRequest)
			return
		}
		decodedEntries[key] = decodedValue
	}

	err := h.Node.ProcessBatchReplication(request.LeaderID, request.Term, decodedEntries)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to handle batch replication: %v", err), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "Batch replication successful")
}
