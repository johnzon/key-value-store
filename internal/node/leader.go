package node

import (
	"encoding/base64"
	"fmt"
	"log"
	"time"
)

// BecomeFollower transitions the node to Follower
func (n *Node) BecomeFollower(leaderID string, term int) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	n.Role = FollowerRole
	n.LeaderID = leaderID
	n.Term = term
	n.IsLeaderActive = true
	fmt.Printf("Node %s became Follower under Leader %s (Term %d)\n", n.ID, leaderID, term)
}

func (n *Node) StartHeartbeat() {
	if n.Role != LeaderRole {
		return
	}

	ticker := time.NewTicker(n.HeartbeatInterval)
	defer ticker.Stop()

	// Worker pool to limit concurrency
	pool := make(chan struct{}, 10) // Max 10 concurrent heartbeats

	for range ticker.C {
		// Check role at the start of each tick
		n.Mutex.Lock()
		if n.Role != LeaderRole {
			n.Mutex.Unlock()
			return // Exit if no longer a Leader
		}
		followers := make(map[string]string)
		for k, v := range n.Followers {
			followers[k] = v
		}
		n.Mutex.Unlock()

		// Send heartbeat to each follower
		for followerID, followerURL := range followers {
			pool <- struct{}{} // Acquire a worker slot
			go func(id, url string) {
				defer func() { <-pool }() // Release worker slot
				if err := n.SendHeartbeat(url); err != nil {
					fmt.Printf("[Heartbeat] Failed to send heartbeat to %s: %v\n", id, err)
				}
			}(followerID, followerURL)
		}
	}
}

func (n *Node) RegisterFollower(nodeId string, nodeUrl string) {
	if n.Role != LeaderRole {
		return
	}
	n.Mutex.Lock()
	defer n.Mutex.Unlock()

	n.Followers[nodeId] = nodeUrl

}

// sendHeartbeat sends a heartbeat message to a follower node.
func (n *Node) SendHeartbeat(url string) error {

	return n.Communicator.SendHeartbeat(url, n.ID, n.Term, n.Followers)
}

// replicateToFollower replicates data to a follower node.
func (n *Node) ReplicateToFollower(url, key string, value []byte) error {
	replicationPayload := map[string]interface{}{
		"leaderID":  n.ID,
		"term":      n.Term,
		"key":       key,
		"value":     base64.StdEncoding.EncodeToString(value),
		"timestamp": time.Now().Unix(),
	}

	pool := make(chan struct{}, 10) // Limit replication to 10 concurrent operations

	pool <- struct{}{}        // acquire worker
	defer func() { <-pool }() // release worker

	_, err := n.Communicator.PostHTTPRequest(fmt.Sprintf("%s/replicate", url), replicationPayload)
	return err
}

// BatchReplicateToFollower replicates a batch of entries to a follower.
func (n *Node) BatchReplicateToFollower(url string, entries map[string][]byte) error {
	replicationPayload := map[string]interface{}{
		"leaderID":  n.ID,
		"term":      n.Term,
		"entries":   entries,
		"timestamp": time.Now().Unix(),
	}

	_, err := n.Communicator.PostHTTPRequest(fmt.Sprintf("%s/batch-replicate", url), replicationPayload)
	return err
}

// StartLeader starts the leader-specific tasks
func (n *Node) StartLeader() {
	log.Println("[Leader] Node started as Leader")

	if !n.CheckShutdownFlag() {
		// Node crashed, need to recover from WAL
		err := n.RecoverFromWAL()
		if err != nil {
			log.Fatalf("failed to recover from WAL: %v", err)
		}
	}
	// If it was a clean shutdown, just proceed
	n.ClearShutdownFlag()
	go n.StartHeartbeat()

}
