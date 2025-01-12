package tests

import (
	"os"
	"testing"

	"keyvalue-store/internal/storage"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDataStore is a mock implementation of the DataStore interface
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

func (m *MockDataStore) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDataStore) Sync() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDataStore) Open() error {
	args := m.Called()
	return args.Error(0)
}

// Open is a mock function to open the data store
func Open(path string) (*MockDataStore, error) {
	store := new(MockDataStore)
	store.On("Open").Return(nil)
	return store, nil
}

// TestStorage_Put tests the Put method
func TestStorage_Put(t *testing.T) {
	file, err := os.CreateTemp("", "bitcask_test")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	store, err := Open(file.Name())
	assert.NoError(t, err)
	defer store.Close()

	key := "testKey"
	value := []byte("testValue")
	err = store.Put(key, value)
	assert.NoError(t, err)

	// Verify the value is written correctly
	readValue, err := store.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, readValue)
}

// TestStorage_Get tests the Get method
func TestStorage_Get(t *testing.T) {
	file, err := os.CreateTemp("", "bitcask_test")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	store, err := Open(file.Name())
	assert.NoError(t, err)
	defer store.Close()

	key := "testKey"
	value := []byte("testValue")
	err = store.Put(key, value)
	assert.NoError(t, err)

	assert.Error(t, err)
	readValue, err := store.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, readValue)

	// Try to get a non-existent key
	_, err = store.Get("nonExistentKey")
	assert.Equal(t, storage.ErrKeyNotFound, err)
}

// TestStorage_Delete tests the Delete method
func TestStorage_Delete(t *testing.T) {
	file, err := os.CreateTemp("", "bitcask_test")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	store, err := Open(file.Name())
	assert.NoError(t, err)
	defer store.Close()

	key := "testKey"
	value := []byte("testValue")
	err = store.Put(key, value)
	assert.NoError(t, err)

	// Delete the key
	err = store.Delete(key)
	assert.NoError(t, err)

	// Verify the key is no longer accessible
	_, err = store.Get(key)
	assert.Equal(t, storage.ErrKeyNotFound, err)
}

// TestStorage_RangeQuery tests the RangeQuery method
func TestStorage_RangeQuery(t *testing.T) {
	file, err := os.CreateTemp("", "bitcask_test")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	store, err := Open(file.Name())
	assert.NoError(t, err)
	defer store.Close()

	// Insert multiple key-value pairs
	data := map[string][]byte{
		"key1": []byte("value1"),
		"key2": []byte("value2"),
		"key3": []byte("value3"),
	}
	for key, value := range data {
		err := store.Put(key, value)
		assert.NoError(t, err)
	}

	// Perform a range query
	results, err := store.RangeQuery("key1", "key3")
	assert.NoError(t, err)
	assert.Equal(t, data, results)
}

// TestStorage_SaveAndLoadIndex tests the SaveIndex and LoadIndex methods
func TestStorage_SaveAndLoadIndex(t *testing.T) {
	file, err := os.CreateTemp("", "bitcask_test")
	assert.NoError(t, err)
	defer os.Remove(file.Name())

	indexFile, err := os.CreateTemp("", "index_test")
	assert.NoError(t, err)
	defer os.Remove(indexFile.Name())

	store, err := Open(file.Name())
	assert.NoError(t, err)
	defer store.Close()

	key := "testKey"
	value := []byte("testValue")
	err = store.Put(key, value)
	assert.NoError(t, err)

	// Save the index to disk
	err = store.SaveIndex(indexFile.Name())
	assert.NoError(t, err)

	// Load the index back
	newStore, err := Open(file.Name())
	assert.NoError(t, err)
	defer newStore.Close()

	err = newStore.LoadIndex(indexFile.Name())
	assert.NoError(t, err)

	// Verify the key is accessible in the new store
	readValue, err := newStore.Get(key)
	assert.NoError(t, err)
	assert.Equal(t, value, readValue)
}
