package tests

import (
	"errors"
	"keyvalue-store/internal/node"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHandleWrite_Success(t *testing.T) {
	mockStore := new(MockDataStore)
	mockWAL := new(MockWALWriter)

	// Setup Node
	node, err := node.NewNode(
		"node1",
		"8080",
		node.LeaderRole, // Role is Leader
		"http://localhost:8080",
		mockStore,
		mockWAL,
		time.Second,
		time.Second*5,
	)
	assert.NoError(t, err)

	key := "key1"
	value := []byte("value1")
	lsn := int64(1)

	// Mock WAL WriteEntry
	mockWAL.On("WriteEntry", "PUT", key, value).Return(lsn, nil)

	// Mock Store Put
	mockStore.On("Put", key, value).Return(nil)

	// Mock SaveCheckpoint
	mockStore.On("Sync").Return(nil)

	// Call HandleWrite
	err = node.HandleWrite(key, value)
	assert.NoError(t, err)

	// Assert Expectations
	mockWAL.AssertExpectations(t)
	mockStore.AssertExpectations(t)
	assert.Equal(t, lsn, node.LastAppliedOffset)
}

func TestHandleWrite_NotLeader(t *testing.T) {
	mockStore := new(MockDataStore)
	mockWAL := new(MockWALWriter)

	// Setup Node with FollowerRole
	node, err := node.NewNode(
		"node1",
		"8080",
		node.FollowerRole, // Role is Follower
		"http://localhost:8080",
		mockStore,
		mockWAL,
		time.Second,
		time.Second*5,
	)
	assert.NoError(t, err)

	key := "key1"
	value := []byte("value1")

	// Call HandleWrite
	err = node.HandleWrite(key, value)
	assert.Error(t, err)
	assert.Equal(t, "Node node1 is not a Leader", err.Error())
}

func TestHandleWrite_EmptyKey(t *testing.T) {
	mockStore := new(MockDataStore)
	mockWAL := new(MockWALWriter)

	// Setup Node
	node, err := node.NewNode(
		"node1",
		"8080",
		node.LeaderRole,
		"http://localhost:8080",
		mockStore,
		mockWAL,
		time.Second,
		time.Second*5,
	)
	assert.NoError(t, err)

	value := []byte("value1")

	// Call HandleWrite with empty key
	err = node.HandleWrite("", value)
	assert.Error(t, err)
	assert.Equal(t, "Key  is not valid", err.Error())
}

func TestHandleWrite_WALWriteError(t *testing.T) {
	mockStore := new(MockDataStore)
	mockWAL := new(MockWALWriter)

	// Setup Node
	node, err := node.NewNode(
		"node1",
		"8080",
		node.LeaderRole,
		"http://localhost:8080",
		mockStore,
		mockWAL,
		time.Second,
		time.Second*5,
	)
	assert.NoError(t, err)

	key := "key1"
	value := []byte("value1")

	// Mock WAL WriteEntry to return an error
	mockWAL.On("WriteEntry", "PUT", key, value).Return(int64(0), errors.New("WAL write error"))

	// Call HandleWrite
	err = node.HandleWrite(key, value)
	assert.Error(t, err)
	assert.Equal(t, "failed to write to WAL: WAL write error", err.Error())

	// Assert Expectations
	mockWAL.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}

func TestHandleWrite_StorePutError(t *testing.T) {
	mockStore := new(MockDataStore)
	mockWAL := new(MockWALWriter)

	// Setup Node
	node, err := node.NewNode(
		"node1",
		"8080",
		node.LeaderRole,
		"http://localhost:8080",
		mockStore,
		mockWAL,
		time.Second,
		time.Second*5,
	)
	assert.NoError(t, err)

	key := "key1"
	value := []byte("value1")
	lsn := int64(1)

	// Mock WAL WriteEntry
	mockWAL.On("WriteEntry", "PUT", key, value).Return(lsn, nil)

	// Mock Store Put to return an error
	mockStore.On("Put", key, value).Return(errors.New("Store put error"))

	// Call HandleWrite
	err = node.HandleWrite(key, value)
	assert.Error(t, err)
	assert.Equal(t, "Store put error", err.Error())

	// Assert Expectations
	mockWAL.AssertExpectations(t)
	mockStore.AssertExpectations(t)
}
