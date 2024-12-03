// raft.go
package raft

import (
	"log"
	"sync"
	"time"
)

type Raft struct {
	ID                string
	State             NodeState
	Term              int
	VotedFor          string
	votesReceived     int
	peers             []string
	lastHeartbeatTime time.Time
	HeartbeatInterval time.Duration
	ElectionTimeout   time.Duration
	stateMutex        sync.Mutex
}

func (r *Raft) becomeLeader() {
	r.State = Leader
	r.Term++
	log.Printf("Node %s became the leader for term %d\n", r.ID, r.Term)
	go r.startHeartbeat()
}

func (r *Raft) startHeartbeat() {
	ticker := time.NewTicker(r.HeartbeatInterval)
	defer ticker.Stop()
	for r.State == Leader {
		select {
		case <-ticker.C:
			r.sendHeartbeats()
		}
	}
}

func (r *Raft) sendHeartbeats() {
	for _, peer := range r.peers {
		// Simulate sending a heartbeat to peer
		log.Printf("Heartbeat sent to %s\n", peer)
	}
}

// Method to handle receiving votes and transitioning to leader if needed
func (r *Raft) startElection() {
	r.State = Candidate
	r.Term++
	r.VotedFor = r.ID
	r.votesReceived = 1 // Vote for self
	log.Printf("Node %s starting election for term %d\n", r.ID, r.Term)

	// Send vote requests to peers
	for _, peer := range r.peers {
		go r.requestVote(peer)
	}
}

func (r *Raft) requestVote(peer string) {
	// Simulate sending a request and receiving a response
	log.Printf("Node %s requesting vote from %s\n", r.ID, peer)
	r.votesReceived++
	if r.votesReceived > len(r.peers)/2 && r.State == Candidate {
		r.becomeLeader()
	}
}
