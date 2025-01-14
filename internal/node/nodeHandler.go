package node

import (
	"keyvalue-store/internal/transaction"
)

// NodeHandler defines the methods that a Node should implement
type NodeHandler interface {

	//Storage Operations
	HandleWrite(key string, value []byte) error
	HandleGet(key string) ([]byte, error)
	HandleDelete(key string) error
	HandleBatchPut(entries map[string][]byte) error
	HandleQueryBatch(startKey, endKey string) (map[string][]byte, error)
	HandleVoteRequest(candidateID string, term int, lastLogIndex int64, lastLogTerm int64) (bool, int)
	BatchWriteToWAL(batch []transaction.WALEntry) error

	//State Management
	SaveCheckpoint() error
	LoadCheckpoint() error
	Shutdown()
	ClearShutdownFlag() error
	CheckShutdownFlag() bool
	MarkShutdown() error
	RecoverFromWAL() error

	//Node as a leader
	BecomeFollower(leaderID string, term int)
	StartHeartbeat()
	RegisterFollower(nodeId string, nodeUrl string)
	SendHeartbeat(url string) error
	ReplicateToFollower(url, key string, value []byte) error
	ProcessBatchReplication(leaderID string, term int, entries map[string][]byte) error
	StartLeader()

	//Node as a follower
	StartFollower()
	BecomeLeader()
	RegisterWithLeader() error
	MonitorLeader()
	ProcessReplication(leaderID string, term int, key string, value []byte, timestamp int64, operation Operation) error

	//Raft Operation
	RequestVote(url string) (bool, error)
	ResetElectionTimer()
	HandleHeartbeat(term int, leaderID string, sourceUrl string, followers map[string]string)

	//Node getters
	IsLeader() bool
	IsFollower() bool
}
