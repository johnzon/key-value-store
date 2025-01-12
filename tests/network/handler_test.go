package tests

import (
	"bytes"
	"encoding/json"
	"keyvalue-store/internal/network"
	"keyvalue-store/internal/node"
	"keyvalue-store/internal/transaction"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type MockNodeHandler struct {
	mock.Mock
}

// Implement Storage Operations
func (m *MockNodeHandler) HandleWrite(key string, value []byte) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *MockNodeHandler) HandleGet(key string) ([]byte, error) {
	args := m.Called(key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockNodeHandler) HandleDelete(key string) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *MockNodeHandler) HandleBatchPut(entries map[string][]byte) error {
	args := m.Called(entries)
	return args.Error(0)
}

func (m *MockNodeHandler) HandleQueryBatch(startKey, endKey string) (map[string][]byte, error) {
	args := m.Called(startKey, endKey)
	return args.Get(0).(map[string][]byte), args.Error(1)
}

func (m *MockNodeHandler) HandleVoteRequest(candidateID string, term int, lastLogIndex int64, lastLogTerm int64) (bool, int) {
	args := m.Called(candidateID, term)
	return args.Bool(0), args.Int(1)
}

func (m *MockNodeHandler) BatchWriteToWAL(batch []transaction.WALEntry) error {
	args := m.Called(batch)
	return args.Error(0)
}

// Implement State Management
func (m *MockNodeHandler) SaveCheckpoint() error {
	return m.Called().Error(0)
}

func (m *MockNodeHandler) LoadCheckpoint() error {
	return m.Called().Error(0)
}

func (m *MockNodeHandler) Shutdown() {
	m.Called()
}

func (m *MockNodeHandler) ClearShutdownFlag() error {
	return m.Called().Error(0)
}

func (m *MockNodeHandler) CheckShutdownFlag() bool {
	return m.Called().Bool(0)
}

func (m *MockNodeHandler) MarkShutdown() error {
	return m.Called().Error(0)
}

func (m *MockNodeHandler) RecoverFromWAL() error {
	return m.Called().Error(0)
}

// Implement Node as a Leader
func (m *MockNodeHandler) BecomeFollower(leaderID string, term int) {
	m.Called(leaderID, term)
}

func (m *MockNodeHandler) StartHeartbeat() {
	m.Called()
}

func (m *MockNodeHandler) RegisterFollower(nodeID string, nodeURL string) {
	m.Called(nodeID, nodeURL)
}

func (m *MockNodeHandler) SendHeartbeat(url string) error {
	return m.Called(url).Error(0)
}

func (m *MockNodeHandler) ReplicateToFollower(url, key string, value []byte) error {
	return m.Called(url, key, value).Error(0)
}

func (m *MockNodeHandler) StartLeader() {
	m.Called()
}

// Implement Node as a Follower
func (m *MockNodeHandler) StartFollower() {
	m.Called()
}

func (m *MockNodeHandler) BecomeLeader() {
	m.Called()
}

func (m *MockNodeHandler) RegisterWithLeader() error {
	return m.Called().Error(0)
}

func (m *MockNodeHandler) MonitorLeader() {
	m.Called()
}

// Implement Raft Operations
func (m *MockNodeHandler) RequestVote(url string) error {
	return m.Called(url).Error(0)
}

func (m *MockNodeHandler) ResetElectionTimer() {
	m.Called()
}
func (m *MockNodeHandler) HandleHeartbeat(term int, leaderID string, sourceURL string) {
	m.Called(term, leaderID, sourceURL)
}

func (m *MockNodeHandler) IsLeader() bool {
	return m.Called().Bool(0)
}

func (m *MockNodeHandler) IsFollower() bool {
	return m.Called().Bool(0)
}

// HandleReplication processes the replication data on the follower node.
func (m *MockNodeHandler) ProcessReplication(leaderID string, term int, key string, value []byte, timestamp int64, operation node.Operation) error {
	return m.Called(leaderID, term, key, value, timestamp, operation).Error(0)
}

// HandleBatchReplication processes batch replication requests from the leader.
func (m *MockNodeHandler) ProcessBatchReplication(leaderID string, term int, entries map[string][]byte) error {
	return m.Called(leaderID, term, entries).Error(0)

}

func TestHandleWrite(t *testing.T) {
	mockNode := new(MockNodeHandler)
	server := &network.Server{Node: mockNode}

	// Define the payload
	payload := map[string]interface{}{
		"key":   "testKey",
		"value": []byte("testValue"),
	}
	payloadBytes, _ := json.Marshal(payload)

	// Mock expectations
	mockNode.On("HandleWrite", "testKey", []byte("testValue")).Return(nil)

	// Create a test request and recorder
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(payloadBytes))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	// Call the handler
	server.HandleWrite(rr, req)

	// Assert the response and mock interactions
	require.Equal(t, http.StatusOK, rr.Code)
	require.Equal(t, "Write successful", rr.Body.String())
	mockNode.AssertCalled(t, "HandleWrite", "testKey", []byte("testValue"))
}
