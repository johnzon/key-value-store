package transaction

import (
	"encoding/gob"
	"os"
	"time"
)

// LogRecord represents a single log entry in the WAL
type LogRecord struct {
	Timestamp time.Time
	Operation string
	Key       string
	Value     []byte
}

// WAL represents the Write-Ahead Log structure
type WAL struct {
	file *os.File
	enc  *gob.Encoder
}

// NewWAL creates a new WAL instance
func NewWAL(filePath string) (*WAL, error) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	return &WAL{
		file: file,
		enc:  gob.NewEncoder(file),
	}, nil
}

// WriteLog writes a log record to the WAL
func (w *WAL) WriteLog(record *LogRecord) error {
	return w.enc.Encode(record)
}

// Close ensures the WAL file is closed properly
func (w *WAL) Close() error {
	return w.file.Close()
}
