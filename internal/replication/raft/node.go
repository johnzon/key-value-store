// node.go
package raft

import (
	"log"
	"math/rand"
	"sync"
	"time"
)

type NodeState int

const (
	Follower NodeState = iota
	Candidate
	Leader
)

type Node struct {
	ID                 string
	State              NodeState
	Term               int
	VotedFor           string
	HeartbeatInterval  time.Duration
	ElectionTimeout    time.Duration
	lastHeartbeatTime  time.Time
	votesReceived      int
	peers              []string
	stateMutex         sync.Mutex
	electionResetEvent chan struct{}
}

func (n *Node) Run() {
	go n.startElectionTimeout()
}

func (n *Node) startElectionTimeout() {
	for {
		select {
		case <-time.After(n.ElectionTimeout + time.Duration(rand.Intn(100))*time.Millisecond):
			n.stateMutex.Lock()
			timeSinceHeartbeat := time.Since(n.lastHeartbeatTime)
			if timeSinceHeartbeat >= n.ElectionTimeout && n.State != Leader {
				n.startElection()
			}
			n.stateMutex.Unlock()
		case <-n.electionResetEvent:
			// Election reset received, restart timer
		}
	}
}

func (n *Node) startElection() {
	n.State = Candidate
	n.Term++
	n.VotedFor = n.ID
	n.votesReceived = 1 // Vote for self
	log.Printf("Node %s starting election for term %d\n", n.ID, n.Term)

	// Send vote requests to peers
	for _, peer := range n.peers {
		go n.requestVote(peer)
	}
}

func (n *Node) requestVote(peer string) {
	// Simulate sending a request and receiving a response
	log.Printf("Node %s requesting vote from %s\n", n.ID, peer)
	n.stateMutex.Lock()
	defer n.stateMutex.Unlock()
	n.votesReceived++
	if n.votesReceived > len(n.peers)/2 && n.State == Candidate {
		n.becomeLeader()
	}
}

func (n *Node) becomeLeader() {
	n.State = Leader
	log.Printf("Node %s became the leader for term %d\n", n.ID, n.Term)
	go n.startHeartbeat()
}

func (n *Node) startHeartbeat() {
	ticker := time.NewTicker(n.HeartbeatInterval)
	defer ticker.Stop()
	for n.State == Leader {
		select {
		case <-ticker.C:
			n.sendHeartbeats()
		}
	}
}

func (n *Node) sendHeartbeats() {
	for _, peer := range n.peers {
		log.Printf("Heartbeat sent to %s\n", peer)
	}
}
