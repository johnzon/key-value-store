package transaction

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// WALWriter defines the interface for writing to the WAL
type WALWriter interface {
	WriteEntry(term int64, operation, key string, value []byte) (int64, error)
	Recover(lastAppliedLSN int64) ([]WALEntry, error)
	Close() error
	GetLastLogIndexAndTerm() (int64, int64)
	SetCommittedIndex(index int64)
}

// WAL represents the Write-Ahead Log structure
type WAL struct {
	File           *os.File
	nextLSN        int64
	Buffer         []WALEntry
	BufferSize     int
	FlushTicker    *time.Ticker
	Mutex          sync.Mutex
	currentTerm    int64 // Raft: Current term
	committedIndex int64 // Raft: Committed index
}

// WALEntry represents an entry in the WAL
type WALEntry struct {
	LSN       int64  // Log Sequence Number
	Term      int64  // Raft term
	Operation string // e.g., "SET", "DELETE"
	Key       string
	Value     []byte
	Timestamp int64 // Timestamp in nanoseconds
}

// NewWAL initializes a new WAL instance
func NewWAL(filePath string, bufferSize int, flushInterval time.Duration) (*WAL, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file: %w", err)
	}

	wal := &WAL{
		File:           file,
		nextLSN:        0,
		Buffer:         make([]WALEntry, 0, bufferSize),
		BufferSize:     bufferSize,
		FlushTicker:    time.NewTicker(flushInterval),
		currentTerm:    0,
		committedIndex: 0,
	}

	// Start periodic flush
	go wal.periodicFlush()

	return wal, nil
}

// WriteEntry adds a new entry to the WAL
func (w *WAL) WriteEntry(term int64, operation, key string, value []byte) (int64, error) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	entry := WALEntry{
		LSN:       w.nextLSN,
		Term:      term,
		Operation: operation,
		Key:       key,
		Value:     value,
		Timestamp: time.Now().UnixNano(),
	}

	w.Buffer = append(w.Buffer, entry)
	w.nextLSN++

	// Flush if buffer is full
	if len(w.Buffer) >= w.BufferSize {
		if err := w.flushBuffer(); err != nil {
			return -1, err
		}
	}

	return entry.LSN, nil
}

// flushBuffer writes buffered entries to the WAL file
func (w *WAL) flushBuffer() error {
	for _, entry := range w.Buffer {
		line := fmt.Sprintf("%d,%d,%s,%s,%s,%d\n",
			entry.LSN, entry.Term, entry.Operation, entry.Key, entry.Value, entry.Timestamp)
		if _, err := w.File.WriteString(line); err != nil {
			return fmt.Errorf("failed to write entry to WAL: %w", err)
		}
	}

	// Sync to disk for durability
	if err := w.File.Sync(); err != nil {
		return fmt.Errorf("failed to sync WAL to disk: %w", err)
	}

	// Clear the buffer
	w.Buffer = w.Buffer[:0]
	return nil
}

// GetLastLogIndexAndTerm returns the last log index and term
func (w *WAL) GetLastLogIndexAndTerm() (int64, int64) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()

	if len(w.Buffer) > 0 {
		lastEntry := w.Buffer[len(w.Buffer)-1]
		return lastEntry.LSN, lastEntry.Term
	}

	return w.nextLSN - 1, 0 // If no entries in buffer, return the previous LSN and term 0
}

// SetCommittedIndex updates the committed index
func (w *WAL) SetCommittedIndex(index int64) {
	w.Mutex.Lock()
	defer w.Mutex.Unlock()
	w.committedIndex = index
}

// Close gracefully closes the WAL
func (w *WAL) Close() error {
	w.FlushTicker.Stop()
	if err := w.flushBuffer(); err != nil {
		return err
	}
	return w.File.Close()
}

// periodicFlush periodically flushes the buffer to disk
func (w *WAL) periodicFlush() {
	for range w.FlushTicker.C {
		w.Mutex.Lock()
		_ = w.flushBuffer()
		w.Mutex.Unlock()
	}
}

// Recover replays the WAL from the given LSN
func (w *WAL) Recover(lastAppliedLSN int64) ([]WALEntry, error) {
	file, err := os.Open(w.File.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to open WAL file for recovery: %w", err)
	}
	defer file.Close()

	var entries []WALEntry
	var entry WALEntry
	decoder := json.NewDecoder(file)

	for {
		if err := decoder.Decode(&entry); err != nil {
			break // End of file
		}

		// Only replay entries after last applied LSN
		if entry.LSN > lastAppliedLSN {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}
