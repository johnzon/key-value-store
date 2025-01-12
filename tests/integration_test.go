package test

import (
	"bytes"
	"encoding/json"
	"keyvalue-store/internal/network"
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

// Mocked methods of NodeHandler
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

func (m *MockNodeHandler) HandleVoteRequest(candidateID string, term int) (bool, int) {
	args := m.Called(candidateID, term)
	return args.Bool(0), args.Int(1)
}

func (m *MockNodeHandler) BatchWriteToWAL(batch []transaction.WALEntry) error {
	args := m.Called(batch)
	return args.Error(0)
}

func (m *MockNodeHandler) SaveCheckpoint() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNodeHandler) LoadCheckpoint() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNodeHandler) Shutdown() {
	m.Called()
}

func (m *MockNodeHandler) ClearShutdownFlag() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNodeHandler) CheckShutdownFlag() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockNodeHandler) MarkShutdown() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNodeHandler) RecoverFromWAL() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNodeHandler) BecomeFollower(leaderID string, term int) {
	m.Called(leaderID, term)
}

func (m *MockNodeHandler) StartHeartbeat() {
	m.Called()
}

func (m *MockNodeHandler) RegisterFollower(nodeId string, nodeUrl string) {
	m.Called(nodeId, nodeUrl)
}

func (m *MockNodeHandler) SendHeartbeat(url string) error {
	args := m.Called(url)
	return args.Error(0)
}

func (m *MockNodeHandler) ReplicateToFollower(url, key string, value []byte) error {
	args := m.Called(url, key, value)
	return args.Error(0)
}

func (m *MockNodeHandler) StartLeader() {
	m.Called()
}

func (m *MockNodeHandler) StartFollower() {
	m.Called()
}

func (m *MockNodeHandler) BecomeLeader() {
	m.Called()
}

func (m *MockNodeHandler) RegisterWithLeader() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockNodeHandler) MonitorLeader() {
	m.Called()
}

func (m *MockNodeHandler) RequestVote(url string) error {
	args := m.Called(url)
	return args.Error(0)
}

func (m *MockNodeHandler) ResetElectionTimer() {
	m.Called()
}

func (m *MockNodeHandler) HandleHeartbeat(term int, leaderID string, sourceUrl string) {
	m.Called(term, leaderID, sourceUrl)
}

func (m *MockNodeHandler) IsLeader() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockNodeHandler) IsFollower() bool {
	args := m.Called()
	return args.Bool(0)
}

// Test setup function
func setupTestServer(mockNode *MockNodeHandler) *httptest.Server {
	server := network.NewServer(mockNode, "8080")
	return httptest.NewServer(server.Router)
}

// Test cases
func TestNodeEndpoints(t *testing.T) {
	mockNode := &MockNodeHandler{}
	ts := setupTestServer(mockNode)
	defer ts.Close()

	// /read endpoint
	t.Run("GET /read", func(t *testing.T) {
		mockNode.On("HandleGet", "key1").Return([]byte("value1"), nil)
		resp, err := http.Get(ts.URL + "/read?key=key1")
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)

		var body []byte
		resp.Body.Read(body)
		require.Equal(t, "value1", string(body))
		mockNode.AssertCalled(t, "HandleGet", "key1")
	})

	// /write endpoint
	t.Run("POST /write", func(t *testing.T) {
		mockNode.On("HandleWrite", "key1", []byte("value1")).Return(nil)
		payload := map[string]interface{}{
			"key":   "key1",
			"value": "value1",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL+"/write", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		mockNode.AssertCalled(t, "HandleWrite", "key1", []byte("value1"))
	})

	// /batchwrite endpoint
	t.Run("POST /batchwrite", func(t *testing.T) {
		mockNode.On("HandleBatchPut", mock.Anything).Return(nil)
		payload := map[string][]byte{
			"key1": []byte("value1"),
			"key2": []byte("value2"),
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL+"/batchwrite", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		mockNode.AssertCalled(t, "HandleBatchPut", mock.Anything)
	})

}

// Test cases for /registerfollower, /vote, and /heartbeat
func TestNodeRoleBasedEndpoints(t *testing.T) {
	mockNode := &MockNodeHandler{}
	ts := setupTestServer(mockNode)
	defer ts.Close()

	// /registerfollower endpoint
	t.Run("POST /register_follower - Success for follower", func(t *testing.T) {
		mockNode.On("IsFollower").Return(true)
		mockNode.On("RegisterFollower", "node1", "http://node1-url").Return(nil)

		payload := map[string]string{
			"nodeId": "node1",
			"url":    "http://node1-url",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL+"/register_follower", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		mockNode.AssertCalled(t, "IsFollower")
		mockNode.AssertCalled(t, "RegisterFollower", "node1", "http://node1-url")
	})

	t.Run("POST /registerfollower - Failure for non-follower", func(t *testing.T) {
		mockNode.On("IsFollower").Return(false)

		payload := map[string]string{
			"nodeId": "node1",
			"url":    "http://node1-url",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL+"/register_follower", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		mockNode.AssertCalled(t, "IsFollower")
	})

	// /vote endpoint
	t.Run("POST /vote - Success for candidate", func(t *testing.T) {
		mockNode.On("IsFollower").Return(false) // Role should not be follower
		mockNode.On("HandleVoteRequest", "candidate1", 2).Return(true, 2)

		payload := map[string]interface{}{
			"candidateId": "candidate1",
			"term":        2,
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL+"/vote", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		mockNode.AssertCalled(t, "HandleVoteRequest", "candidate1", 2)
	})

	t.Run("POST /vote - Failure for non-candidate", func(t *testing.T) {
		mockNode.On("IsFollower").Return(true) // Role is follower, not candidate

		payload := map[string]interface{}{
			"candidateId": "candidate1",
			"term":        2,
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL+"/vote", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
	})

	// /heartbeat endpoint
	t.Run("POST /heartbeat - Success for leader", func(t *testing.T) {
		mockNode.On("IsLeader").Return(true)
		mockNode.On("SendHeartbeat", "http://node2-url").Return(nil)

		payload := map[string]string{
			"url": "http://node2-url",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL+"/heartbeat", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, resp.StatusCode)
		mockNode.AssertCalled(t, "IsLeader")
		mockNode.AssertCalled(t, "SendHeartbeat", "http://node2-url")
	})

	t.Run("POST /heartbeat - Failure for non-leader", func(t *testing.T) {
		mockNode.On("IsLeader").Return(false)

		payload := map[string]string{
			"url": "http://node2-url",
		}
		body, _ := json.Marshal(payload)

		resp, err := http.Post(ts.URL+"/heartbeat", "application/json", bytes.NewBuffer(body))
		require.NoError(t, err)
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		mockNode.AssertCalled(t, "IsLeader")
	})
}
