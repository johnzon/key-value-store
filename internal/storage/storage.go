package storage

import (
	"errors"
	"keyvalue-store/internal/transaction"
	"os"
	"sort"
	"sync"
	"time"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

type record struct {
	key    string
	offset int64
}

// Storage represents the key-value storage system
type Storage struct {
	dataFile        *os.File
	index           []record // Sorted slice of key-offset pairs
	indexMutex      sync.RWMutex
	wal             *transaction.WAL
	snapshotManager *transaction.SnapshotManager
}

// NewStorage initializes the storage system
func NewStorage(filePath string, walPath string, snapshotPath string) (*Storage, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	wal, err := transaction.NewWAL(walPath)
	if err != nil {
		return nil, err
	}

	snapshotManager := transaction.NewSnapshotManager(snapshotPath)

	return &Storage{
		dataFile:        file,
		index:           []record{},
		wal:             wal,
		snapshotManager: snapshotManager,
	}, nil
}

// Put stores a key-value pair
func (s *Storage) Put(key string, value []byte) error {
	s.indexMutex.Lock()
	defer s.indexMutex.Unlock()

	// Log the operation to WAL
	logRecord := &transaction.LogRecord{
		Timestamp: time.Now(),
		Operation: "PUT",
		Key:       key,
		Value:     value,
	}
	err := s.wal.WriteLog(logRecord)
	if err != nil {
		return err
	}

	// Append the key-value pair to the data file
	offset, err := s.dataFile.Seek(0, os.SEEK_END)
	if err != nil {
		return err
	}

	dataRecord := append([]byte(key+"|"), value...)
	dataRecord = append(dataRecord, '\n') // Add newline for separation
	_, err = s.dataFile.Write(dataRecord)
	if err != nil {
		return err
	}

	// Insert the key in sorted order in the index
	index := sort.Search(len(s.index), func(i int) bool {
		return s.index[i].key >= key
	})

	// Replace if key exists, otherwise insert
	if index < len(s.index) && s.index[index].key == key {
		// If the key already exists, update the offset
		s.index[index].offset = offset
	} else {
		// Insert a new record (key and offset) into the sorted index
		newRecord := record{key: key, offset: offset}
		s.index = append(s.index[:index], append([]record{newRecord}, s.index[index:]...)...)
	}

	return nil
}

// BatchPut stores multiple key-value pairs atomically
func (s *Storage) BatchPut(keys []string, values [][]byte) error {
	if len(keys) != len(values) {
		return errors.New("keys and values must have the same length")
	}

	s.indexMutex.Lock()
	defer s.indexMutex.Unlock()

	for i, key := range keys {
		// Append the key-value pair to the data file
		offset, err := s.dataFile.Seek(0, os.SEEK_END)
		if err != nil {
			return err
		}

		dataRecord := append([]byte(key+"|"), values[i]...)
		dataRecord = append(dataRecord, '\n') // Add newline for separation
		_, err = s.dataFile.Write(dataRecord)
		if err != nil {
			return err
		}

		// Insert key in sorted order
		index := sort.Search(len(s.index), func(i int) bool {
			return s.index[i].key >= key
		})

		// Replace if key exists, otherwise insert
		if index < len(s.index) && s.index[index].key == key {
			s.index[index].offset = offset
		} else {
			s.index = append(s.index[:index], append([]record{{key, offset}}, s.index[index:]...)...)
		}
	}

	return nil
}

// Read retrieves the value associated with a key
func (s *Storage) Read(key string) ([]byte, error) {
	s.indexMutex.RLock()
	defer s.indexMutex.RUnlock()

	// Binary search for the key
	index := sort.Search(len(s.index), func(i int) bool {
		return s.index[i].key >= key
	})

	if index == len(s.index) || s.index[index].key != key {
		return nil, ErrKeyNotFound
	}

	offset := s.index[index].offset
	// Seek to the offset and read the data
	s.dataFile.Seek(offset, os.SEEK_SET)

	buf := make([]byte, 1024) // Assume max size of a record is 1024 bytes
	n, err := s.dataFile.Read(buf)
	if err != nil {
		return nil, err
	}

	// Extract the value from the record
	record := buf[:n]
	sepIndex := len(key) + 1 // key|value format, +1 for the '|'
	if string(record[:sepIndex-1]) != key {
		return nil, ErrKeyNotFound
	}
	return record[sepIndex:], nil
}

// ReadKeyRange retrieves all key-value pairs within the specified key range [startKey, endKey].
func (s *Storage) ReadKeyRange(startKey, endKey string) (map[string][]byte, error) {
	s.indexMutex.RLock()
	defer s.indexMutex.RUnlock()

	results := make(map[string][]byte)

	// Find the start index using binary search
	startIndex := sort.Search(len(s.index), func(i int) bool {
		return s.index[i].key >= startKey
	})

	// Iterate over the sorted keys in range
	for i := startIndex; i < len(s.index) && s.index[i].key <= endKey; i++ {
		offset := s.index[i].offset

		// Read the value from the file
		s.dataFile.Seek(offset, os.SEEK_SET)
		buf := make([]byte, 1024)
		n, err := s.dataFile.Read(buf)
		if err != nil {
			return nil, err
		}

		record := buf[:n]
		key := s.index[i].key
		sepIndex := len(key) + 1
		if string(record[:sepIndex-1]) == key {
			results[key] = record[sepIndex:]
		}
	}

	return results, nil
}

// Delete removes a key from the index (soft delete)
func (s *Storage) Delete(key string) error {
	s.indexMutex.Lock()
	defer s.indexMutex.Unlock()

	index := sort.Search(len(s.index), func(i int) bool {
		return s.index[i].key >= key
	})

	if index == len(s.index) || s.index[index].key != key {
		return ErrKeyNotFound
	}

	s.index = append(s.index[:index], s.index[index+1:]...)
	return nil
}

// Close ensures the file handle is properly closed
func (s *Storage) Close() error {
	return s.dataFile.Close()
}
