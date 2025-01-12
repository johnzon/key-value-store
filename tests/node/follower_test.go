package tests

import (
	"sync"
	"testing"
	"time"

	"keyvalue-store/internal/node"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockCommunicator struct {
	mock.Mock
}

func (m *MockCommunicator) SendHeartbeat(url string, nodeID string, term int, followers map[string]string) error {
	args := m.Called(url, nodeID, term)
	return args.Error(0)
}

func (m *MockCommunicator) PostHTTPRequest(url string, payload map[string]interface{}) ([]byte, error) {
	args := m.Called(url, payload)
	return args.Get(0).([]byte), args.Error(0)
}

func setupTestNode(role node.NodeRole, heartbeatTimeout, electionTimeout time.Duration) (*node.Node, *MockDataStore, *MockWALWriter, error) {
	mockStore := new(MockDataStore)
	mockWAL := new(MockWALWriter)

	n, err := node.NewNode(
		"node1",
		"8080",
		role,
		"http://localhost:8080",
		mockStore,
		mockWAL,
		heartbeatTimeout,
		electionTimeout,
	)
	return n, mockStore, mockWAL, err
}

func TestNode_StartFollower(t *testing.T) {
	n, _, _, err := setupTestNode(node.FollowerRole, time.Second, time.Second*5)
	assert.NoError(t, err)

	// Mock RegisterWithLeader behavior
	mockCommunicator := new(MockCommunicator)
	mockCommunicator.On("PostHTTPRequest", "http://localhost:8080/register_follower", mock.Anything).
		Return(nil)
	n.Communicator = mockCommunicator

	n.StartFollower()
	assert.Equal(t, node.FollowerRole, n.Role, "Node should remain a follower after StartFollower")
}

func TestNode_RegisterWithLeader(t *testing.T) {
	mockStore := &MockDataStore{}
	mockWAL := &MockWALWriter{}
	mockCommunicator := &MockCommunicator{}

	node, err := node.NewNode(
		"node1",
		"8080",
		node.FollowerRole,
		"http://localhost:8080",
		mockStore,
		mockWAL,
		time.Second,
		time.Second*5,
	)
	assert.NoError(t, err)

	node.Communicator = mockCommunicator

	mockCommunicator.On("PostHTTPRequest", "http://localhost:8080/register_follower", mock.Anything).
		Return(nil)

	err = node.RegisterWithLeader()
	assert.NoError(t, err)

	mockCommunicator.AssertExpectations(t)
}

func TestNode_HandleHeartbeat(t *testing.T) {
	n, _, _, err := setupTestNode(node.FollowerRole, time.Second, time.Second*5)
	assert.NoError(t, err)

	n.HandleHeartbeat(2, "leader1", "http://localhost:9090", map[string]string{"node1": "http://localhost:8081"})

	assert.Equal(t, "leader1", n.LeaderID, "LeaderID should be updated")
	assert.Equal(t, "http://localhost:9090", n.LeaderURL, "LeaderURL should be updated")
	assert.Equal(t, 2, n.Term, "Term should be updated")
}

func TestNode_MonitorLeader_TransitionToCandidate(t *testing.T) {
	n, _, _, err := setupTestNode(node.FollowerRole, time.Second, time.Millisecond*500)
	assert.NoError(t, err)

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		n.MonitorLeader()
		wg.Done()
	}()

	time.Sleep(time.Second * 1) // Allow time for leader to timeout

	n.Mutex.Lock()
	assert.Equal(t, node.CandidateRole, n.Role, "Node should transition to Candidate after leader timeout")
	n.Mutex.Unlock()

	n.ElectionTimer.Stop()
	wg.Wait()
}
