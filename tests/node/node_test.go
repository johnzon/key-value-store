package tests

import (
	"errors"
	"keyvalue-store/internal/node"
	"keyvalue-store/internal/transaction"

	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock DataStore
type MockDataStore struct {
	mock.Mock
}

func (m *MockDataStore) Put(key string, value []byte) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *MockDataStore) Delete(key string) error {
	args := m.Called(key)
	return args.Error(0)
}

func (m *MockDataStore) BatchPut(entries map[string][]byte) error {
	args := m.Called(entries)
	return args.Error(0)
}

func (m *MockDataStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDataStore) Get(key string) ([]byte, error) {
	args := m.Called(key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockDataStore) RangeQuery(startKey, endKey string) (map[string][]byte, error) {
	args := m.Called(startKey, endKey)
	return args.Get(0).(map[string][]byte), args.Error(1)
}

func (m *MockDataStore) SaveIndex(indexPath string) error {
	args := m.Called(indexPath)
	return args.Error(0)
}

func (m *MockDataStore) LoadIndex(indexPath string) error {
	args := m.Called(indexPath)
	return args.Error(0)
}

func (m *MockDataStore) Sync() error {
	args := m.Called()
	return args.Error(0)
}

// Mock WALWriter
type MockWALWriter struct {
	mock.Mock
}

func (m *MockWALWriter) WriteEntry(term int64, operation, key string, value []byte) (int64, error) {
	args := m.Called(term, operation, key, value)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockWALWriter) Recover(lastAppliedLSN int64) ([]transaction.WALEntry, error) {
	args := m.Called(lastAppliedLSN)
	return args.Get(0).([]transaction.WALEntry), args.Error(1)
}

func (m *MockWALWriter) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockWALWriter) GetLastLogIndexAndTerm() (int64, int64) {
	args := m.Called()
	return args.Get(0).(int64), args.Get(1).(int64)
}

func (m *MockWALWriter) SetCommittedIndex(index int64) {
	m.Called()
}

func TestRecoverFromWAL(t *testing.T) {
	mockStore := new(MockDataStore)
	mockWAL := new(MockWALWriter)

	node, err := node.NewNode("node1", "8080", node.FollowerRole, "http://localhost:8080", mockStore, mockWAL, time.Second, time.Second*5)
	assert.NoError(t, err)

	entries := []transaction.WALEntry{
		{Operation: "PUT", Key: "key1", Value: []byte("value1"), LSN: 1},
		{Operation: "DELETE", Key: "key2", LSN: 2},
	}

	mockWAL.On("Recover", int64(0)).Return(entries, nil)
	mockStore.On("Put", "key1", []byte("value1")).Return(nil)
	mockStore.On("Delete", "key2").Return(nil)

	err = node.RecoverFromWAL()
	assert.NoError(t, err)
	assert.Equal(t, int64(2), node.LastAppliedOffset)
	mockStore.AssertExpectations(t)
	mockWAL.AssertExpectations(t)
}

func TestRecoverFromWAL_Error(t *testing.T) {
	mockStore := new(MockDataStore)
	mockWAL := new(MockWALWriter)

	node, err := node.NewNode("node1", "8080", node.FollowerRole, "http://localhost:8080", mockStore, mockWAL, time.Second, time.Second*5)
	assert.NoError(t, err)

	mockWAL.On("Recover", int64(0)).Return(nil, errors.New("recover error"))

	err = node.RecoverFromWAL()
	assert.Error(t, err)
	assert.Equal(t, "failed to recover WAL: recover error", err.Error())
	mockWAL.AssertExpectations(t)
}
