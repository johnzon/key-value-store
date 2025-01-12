package storage

import (
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/google/btree"
)

// DataStore defines the interface for the storage operations
type DataStore interface {
	Put(key string, value []byte) error
	Delete(key string) error
	BatchPut(entries map[string][]byte) error
	Get(key string) ([]byte, error)
	RangeQuery(startKey, endKey string) (map[string][]byte, error)
	SaveIndex(indexPath string) error
	LoadIndex(indexPath string) error
	Close() error
	Sync() error
}

type Storage struct {
	file        *os.File
	index       map[string]int64 // In-memory index (key -> file offset)
	sortedIndex *btree.BTree     //secondary index for range queries
	writeMutex  sync.Mutex       // Ensures thread-safe writes
	writePos    int64            // Current write position in file
}

// a secondary index sorted for effective range query
type btreeItem struct {
	key    string
	offset int64
}

var (
	ErrKeyNotFound = errors.New("key not found")
)

// Less compares two btree items by key
func (b btreeItem) Less(than btree.Item) bool {
	return b.key < than.(btreeItem).key
}

// Open initializes a new Bitcask storage instance and rebuilds the index from the log
func Open(path string) (*Storage, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	b := &Storage{
		file:        file,
		index:       make(map[string]int64),
		sortedIndex: btree.New(2),
	}

	if err := b.rebuildIndex(); err != nil {
		return nil, err
	}

	return b, nil
}

// rebuildIndex reconstructs the in-memory index from the append-only log
func (b *Storage) rebuildIndex() error {
	b.writePos = 0

	for {
		offset := b.writePos

		keyLenBuf := make([]byte, 8)
		valueLenBuf := make([]byte, 8)
		if _, err := b.file.ReadAt(keyLenBuf, offset); err != nil {
			break
		}
		if _, err := b.file.ReadAt(valueLenBuf, offset+8); err != nil {
			break
		}

		keyLen := binary.BigEndian.Uint64(keyLenBuf)
		valueLen := binary.BigEndian.Uint64(valueLenBuf)

		totalLen := 16 + int64(keyLen) + int64(valueLen)
		if totalLen <= 0 {
			log.Println("Corrupted entry detected, stopping index rebuild")
			break
		}

		key := make([]byte, keyLen)
		if _, err := b.file.ReadAt(key, offset+16); err != nil {
			break
		}

		b.index[string(key)] = offset
		b.writePos += totalLen
	}

	return nil
}

// Put writes a key-value pair to the append-only log
func (b *Storage) Put(key string, value []byte) error {
	b.writeMutex.Lock()
	defer b.writeMutex.Unlock()

	// Prepare entry: key length + value length + key + value
	entry := make([]byte, 8+8+len(key)+len(value))
	binary.BigEndian.PutUint64(entry[0:8], uint64(len(key)))
	binary.BigEndian.PutUint64(entry[8:16], uint64(len(value)))
	copy(entry[16:16+len(key)], key)
	copy(entry[16+len(key):], value)

	// Write entry to file
	offset := b.writePos
	n, err := b.file.Write(entry)
	if err != nil {
		return err
	}

	b.writePos += int64(n)

	// Update in-memory index
	b.index[key] = offset

	b.sortedIndex.ReplaceOrInsert(btreeItem{key, offset})
	return nil
}

// Delete removes a key-value pair from the storage.
func (b *Storage) Delete(key string) error {
	b.writeMutex.Lock()
	defer b.writeMutex.Unlock()

	// Check if the key exists
	offset, exists := b.index[key]
	if !exists {
		return fmt.Errorf("key %s not found", key)
	}

	// Prepare delete marker entry: 'DEL' + key length + key
	entry := make([]byte, 3+8+len(key))
	copy(entry[0:3], "DEL")
	binary.BigEndian.PutUint64(entry[3:11], uint64(len(key)))
	copy(entry[11:], key)

	// Write delete marker to file
	n, err := b.file.Write(entry)
	if err != nil {
		return err
	}

	b.writePos += int64(n)

	// Remove from in-memory index
	delete(b.index, key)
	b.sortedIndex.Delete(btreeItem{key, offset})

	return nil
}

func (b *Storage) BatchPut(entries map[string][]byte) error {
	b.writeMutex.Lock()
	defer b.writeMutex.Unlock()

	var batchData []byte
	var offsetUpdates []struct {
		key    string
		offset int64
	}

	// Prepare all entries and collect them for batch insertion
	var sortedEntries []btree.Item

	for key, value := range entries {
		// Prepare each entry: key length + value length + key + value
		entry := make([]byte, 8+8+len(key)+len(value))
		binary.BigEndian.PutUint64(entry[0:8], uint64(len(key)))
		binary.BigEndian.PutUint64(entry[8:16], uint64(len(value)))
		copy(entry[16:16+len(key)], key)
		copy(entry[16+len(key):], value)

		// Append entry to batch data
		batchData = append(batchData, entry...)
		b.writePos += int64(len(entry)) // Update write position for the next entry

		// Store the offset update for each key
		offsetUpdates = append(offsetUpdates, struct {
			key    string
			offset int64
		}{key: key, offset: b.writePos - int64(len(entry))})

		// Prepare entry for sorted index
		sortedEntries = append(sortedEntries, btreeItem{key, b.writePos - int64(len(entry))})
	}

	// Write all entries at once to the file
	_, err := b.file.Write(batchData)
	if err != nil {
		return err
	}

	// Update the sorted index in bulk
	for _, entry := range sortedEntries {
		b.sortedIndex.ReplaceOrInsert(entry)
	}

	// Update the in-memory index with all offsets in one go
	for _, update := range offsetUpdates {
		b.index[update.key] = update.offset
	}

	return nil
}

// Get retrieves a value by key
func (b *Storage) Get(key string) ([]byte, error) {
	offset, exists := b.index[key]
	if !exists {
		return nil, ErrKeyNotFound
	}
	return b.readValueAtOffset(offset)
}

// RangeQuery retrieves all key-value pairs between startKey and endKey (inclusive)
func (b *Storage) RangeQuery(startKey, endKey string) (map[string][]byte, error) {
	result := make(map[string][]byte)

	// Iterate over the in-memory index to find all keys in the range [startKey, endKey]
	for key, offset := range b.index {
		if key >= startKey && key <= endKey {
			// Key is within the range, retrieve its value from the file at the offset
			value, err := b.readValueAtOffset(offset)
			if err != nil {
				return nil, err
			}
			result[key] = value
		}
	}

	return result, nil
}

// readValueAtOffset reads the value of a key from the file at the given offset
func (b *Storage) readValueAtOffset(offset int64) ([]byte, error) {
	// Move the file pointer to the offset of the key-value pair
	b.file.Seek(offset, 0)

	// Read value length
	valueLenBuf := make([]byte, 8)
	if _, err := b.file.Read(valueLenBuf); err != nil {
		return nil, err
	}

	valueLen := binary.BigEndian.Uint64(valueLenBuf)

	// Read the value corresponding to the key
	value := make([]byte, valueLen)
	if _, err := b.file.Read(value); err != nil {
		return nil, err
	}

	return value, nil
}

// SaveIndex persists the in-memory index to a file on disk.
func (b *Storage) SaveIndex(indexPath string) error {
	// Open the index file for writing
	indexFile, err := os.OpenFile(indexPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer indexFile.Close()

	// Write the index to the file
	for key, offset := range b.index {
		// Write the key length and key itself
		keyLen := uint64(len(key))
		if err := binary.Write(indexFile, binary.BigEndian, keyLen); err != nil {
			return err
		}
		if _, err := indexFile.Write([]byte(key)); err != nil {
			return err
		}

		// Write the offset
		if err := binary.Write(indexFile, binary.BigEndian, offset); err != nil {
			return err
		}
	}

	return nil
}

// LoadIndex loads the index from a file on startup.
func (b *Storage) LoadIndex(indexPath string) error {
	// Open the index file for reading
	indexFile, err := os.OpenFile(indexPath, os.O_RDONLY, 0644)
	if err != nil {
		return err
	}
	defer indexFile.Close()

	// Read the index from the file
	for {
		var keyLen uint64
		if err := binary.Read(indexFile, binary.BigEndian, &keyLen); err != nil {
			break // End of file
		}

		// Read the key
		key := make([]byte, keyLen)
		if _, err := indexFile.Read(key); err != nil {
			return err
		}

		// Read the offset
		var offset int64
		if err := binary.Read(indexFile, binary.BigEndian, &offset); err != nil {
			return err
		}

		// Update the in-memory index
		b.index[string(key)] = offset
	}

	return nil
}

// Close gracefully shuts down the Bitcask storage
func (b *Storage) Close() error {
	return b.file.Close()
}

// Sync ensures all writes are flushed to disk
func (b *Storage) Sync() error {
	return b.file.Sync()
}
