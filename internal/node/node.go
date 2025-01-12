package node

import (
	"fmt"
	"keyvalue-store/internal/storage"
	"keyvalue-store/internal/transaction"
	"os"
	"strconv"
	"sync"
	"time"
)

// NodeRole defines the role of a node in the cluster
type NodeRole string

const (
	LeaderRole    NodeRole = "Leader"
	FollowerRole  NodeRole = "Follower"
	CandidateRole NodeRole = "Candidate"
)

type Operation string

const (
	PUT    Operation = "PUT"
	DELETE Operation = "DELETE"
)

// ShutdownFlagFile is the file that marks a clean shutdown.
const ShutdownFlagFile = "shutdown.flag"

// Node represents a single node in the Raft cluster
type Node struct {
	ID                string
	Role              NodeRole
	URL               string
	Port              string
	LeaderID          string
	LeaderURL         string
	Term              int
	LastHeartbeat     time.Time
	HeartbeatTimeout  time.Duration
	HeartbeatInterval time.Duration
	LastAppliedOffset int64

	// Components
	Store        storage.DataStore
	Communicator NodeCommunicator //communicator interface
	WAL          transaction.WALWriter

	// Raft-specific properties
	Mutex          sync.Mutex
	Followers      map[string]string // followerID -> URL
	ElectionTimer  *time.Timer
	IsLeaderActive bool
	VotedFor       string
}

//var _ NodeInterface = (*Node)(nil)

// NewNode initializes a new Node
func NewNode(id string, port string, role NodeRole, URL string, store storage.DataStore, wal transaction.WALWriter, heartbeatInterval, heartbeatTimeout time.Duration) (*Node, error) {

	node := &Node{
		ID:                id,
		Role:              role,
		URL:               URL,
		Port:              port,
		Term:              0,
		LastHeartbeat:     time.Now(),
		HeartbeatTimeout:  heartbeatTimeout,
		HeartbeatInterval: heartbeatInterval,
		Store:             store,
		Communicator:      NewHTTPCommunicator(),
		Followers:         make(map[string]string),
		WAL:               wal,
	}

	return node, nil
}

func (n *Node) IsLeader() bool {
	return n.Role == LeaderRole
}

func (n *Node) IsFollower() bool {
	return n.Role == FollowerRole
}

func (n *Node) BatchWriteToWAL(batch []transaction.WALEntry) error {
	for _, record := range batch {
		if _, err := n.WAL.WriteEntry(int64(n.Term), record.Operation, record.Key, record.Value); err != nil {
			return err
		}
	}
	return nil
}

// SaveCheckpoint persists the last applied offset
func (n *Node) SaveCheckpoint() error {
	return os.WriteFile("checkpoint.meta", []byte(fmt.Sprintf("%d", n.LastAppliedOffset)), 0644)
}

// LoadCheckpoint loads the last applied offset
func (n *Node) LoadCheckpoint() error {
	data, err := os.ReadFile("checkpoint.meta")
	if os.IsNotExist(err) {
		// No checkpoint file, assume fresh start
		n.LastAppliedOffset = 0
		return nil
	}
	if err != nil {
		return fmt.Errorf("failed to load checkpoint file: %v", err)
	}

	offset, err := strconv.ParseInt(string(data), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse checkpoint offset: %v", err)
	}

	n.LastAppliedOffset = offset
	return nil
}

func (n *Node) RecoverFromWAL() error {
	// Read last applied LSN from storage/checkpoint
	lastAppliedLSN := n.LastAppliedOffset

	// Recover from WAL
	records, err := n.WAL.Recover(lastAppliedLSN)
	if err != nil {
		return fmt.Errorf("failed to recover WAL: %v", err)
	}

	for _, entry := range records {
		switch entry.Operation {
		case "PUT":
			if err := n.Store.Put(entry.Key, entry.Value); err != nil {
				return fmt.Errorf("failed to apply PUT during recovery: %v", err)
			}
		case "DELETE":
			if err := n.Store.Delete(entry.Key); err != nil {
				return fmt.Errorf("failed to apply DELETE during recovery: %v", err)
			}
		default:
			return fmt.Errorf("unknown operation %s in WAL entry", entry.Operation)
		}

		// Update last applied LSN
		n.LastAppliedOffset = entry.LSN
	}

	// Adjust node role if needed
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	if n.Role != LeaderRole {
		n.Role = FollowerRole
	}
	fmt.Println("Recovered state from WAL successfully.")
	return nil
}

// MarkShutdown writes a shutdown flag to the disk.
func (n *Node) MarkShutdown() error {
	file, err := os.Create(ShutdownFlagFile)
	if err != nil {
		return fmt.Errorf("failed to create shutdown flag: %v", err)
	}
	defer file.Close()
	return nil
}

// CheckShutdownFlag checks if the shutdown flag exists.
func (n *Node) CheckShutdownFlag() bool {
	_, err := os.Stat(ShutdownFlagFile)
	return err == nil // If no error, the flag exists, indicating a clean shutdown
}

// ClearShutdownFlag deletes the shutdown flag after recovery.
func (n *Node) ClearShutdownFlag() error {
	return os.Remove(ShutdownFlagFile)
}

// Graceful Shutdown
func (n *Node) Shutdown() {
	n.MarkShutdown()
	n.WAL.Close()
	fmt.Println("Node shut down gracefully.")
}

func (n *Node) HandleVoteRequest(candidateID string, term int, lastLogIndex int64, lastLogTerm int64) (bool, int) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	currentIndex, currentTerm := n.WAL.GetLastLogIndexAndTerm()

	// Check term and log state
	if term < n.Term ||
		(lastLogTerm < currentTerm ||
			(lastLogTerm == currentTerm) && lastLogIndex < currentIndex) {
		return false, n.Term
	}
	// Grant vote and update state
	n.Term = term
	n.VotedFor = candidateID

	return true, n.Term
}
