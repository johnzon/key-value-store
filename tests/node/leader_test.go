package tests

import (
	"keyvalue-store/internal/node"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestNode_StartHeartbeat(t *testing.T) {
	mockCommunicator := &MockCommunicator{}
	node, err := node.NewNode(
		"node1",
		"8080",
		node.LeaderRole,
		"http://localhost:8080",
		nil,
		nil,
		time.Millisecond*100,
		time.Second*5,
	)
	assert.NoError(t, err)

	node.Communicator = mockCommunicator
	node.Followers = map[string]string{
		"follower1": "http://localhost:8081",
		"follower2": "http://localhost:8082",
	}

	mockCommunicator.On("SendHeartbeat", "http://localhost:8081", "node1", mock.Anything).Return(nil)
	mockCommunicator.On("SendHeartbeat", "http://localhost:8082", "node1", mock.Anything).Return(nil)

	// Start the heartbeat
	go node.StartHeartbeat()

	time.Sleep(300 * time.Millisecond) // Allow some time for heartbeats to be sent

	mockCommunicator.AssertCalled(t, "SendHeartbeat", "http://localhost:8081", "node1", mock.Anything)
	mockCommunicator.AssertCalled(t, "SendHeartbeat", "http://localhost:8082", "node1", mock.Anything)
}

func TestNode_RegisterFollower(t *testing.T) {
	node, err := node.NewNode(
		"node1",
		"8080",
		node.LeaderRole,
		"http://localhost:8080",
		nil,
		nil,
		time.Millisecond*100,
		time.Second*5,
	)
	assert.NoError(t, err)

	assert.Empty(t, node.Followers)

	node.RegisterFollower("follower1", "http://localhost:8081")

	node.Mutex.Lock()
	defer node.Mutex.Unlock()

	assert.Equal(t, 1, len(node.Followers))
	assert.Equal(t, "http://localhost:8081", node.Followers["follower1"])
}

func TestNode_ReplicateToFollower(t *testing.T) {
	mockCommunicator := &MockCommunicator{}
	node, err := node.NewNode(
		"node1",
		"8080",
		node.LeaderRole,
		"http://localhost:8080",
		nil,
		nil,
		time.Millisecond*100,
		time.Second*5,
	)
	assert.NoError(t, err)

	node.Communicator = mockCommunicator

	replicationPayload := map[string]interface{}{
		"leaderID":  "node1",
		"term":      node.Term,
		"key":       "testKey",
		"value":     mock.Anything,
		"timestamp": mock.Anything,
	}

	mockCommunicator.On("PostHTTPRequest", "http://localhost:8081/replicate", replicationPayload).Return(nil)

	err = node.ReplicateToFollower("http://localhost:8081", "testKey", []byte("testValue"))
	assert.NoError(t, err)

	mockCommunicator.AssertExpectations(t)
}

func TestNode_StartLeader(t *testing.T) {
	mockStore := &MockDataStore{}
	mockWAL := &MockWALWriter{}
	mockCommunicator := &MockCommunicator{}

	node, err := node.NewNode(
		"node1",
		"8080",
		node.LeaderRole,
		"http://localhost:8080",
		mockStore,
		mockWAL,
		time.Millisecond*100,
		time.Second*5,
	)
	assert.NoError(t, err)

	node.Communicator = mockCommunicator

	mockWAL.On("Recover").Return(nil)
	mockStore.On("ClearShutdownFlag").Return(nil)

	go node.StartLeader()

	time.Sleep(200 * time.Millisecond) // Allow leader tasks to initialize

	mockWAL.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}
