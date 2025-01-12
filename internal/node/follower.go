package node

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// StartFollower starts the follower-specific tasks
func (n *Node) StartFollower() {
	log.Println("[Follower] Node started as Follower")

	err := n.RegisterWithLeader()
	if err != nil {
		log.Fatalf("[Follower] Failed to register with leader: %v", err)
	}

	go n.MonitorLeader()
}

// BecomeLeader transitions the node to Leader
func (n *Node) BecomeLeader() {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	n.Role = LeaderRole
	n.LeaderID = n.ID
	n.Term++
	n.IsLeaderActive = true
	fmt.Printf("Node %s became Leader (Term %d)\n", n.ID, n.Term)
}

func (n *Node) HandleHeartbeat(term int, leaderID string, sourceUrl string, followers map[string]string) {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()

	if term > n.Term {
		n.BecomeFollower(leaderID, term)
	} else {
		n.LeaderID = leaderID
		n.LeaderURL = sourceUrl
	}
	n.Followers = followers
	n.LastHeartbeat = time.Now()
}

func (n *Node) RegisterWithLeader() error {
	// Create the heartbeat request
	heartbeatReq := map[string]interface{}{
		"NodeID": n.ID,
		"Term":   n.Term,
		"URL":    n.URL,
	}
	_, err := n.Communicator.PostHTTPRequest(n.LeaderURL+"/register_follower", heartbeatReq)
	if err != nil {
		return fmt.Errorf("error sending request to leader: %v", err)
	}
	return nil
}

// startElection begins a new leader election and returns true if the node becomes the leader.
func (n *Node) startElection() bool {
	n.Mutex.Lock()

	// Transition to Candidate role
	n.Term++
	n.LeaderID = "" // Clear current leader
	n.Role = CandidateRole
	n.VotedFor = n.ID // Vote for self

	currentTerm := n.Term
	fmt.Printf("[Node %s] Starting election for term %d\n", n.ID, currentTerm)

	n.Mutex.Unlock()

	// Initialize vote count
	var voteMutex sync.Mutex
	votes := 1 // Self-vote
	retries := 3

	// Context for election timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Request votes from followers
	for followerID, followerURL := range n.Followers {
		go func(followerID, followerURL string) {
			var err error
			var voteGranted bool
			for i := 0; i < retries; i++ {
				// Break if election timed out
				if ctx.Err() != nil {
					return
				}

				voteGranted, err = n.RequestVote(followerURL)
				if err != nil {
					//error while requesting for vote. should we retry?
					//TODO:
				}
				if voteGranted {
					voteMutex.Lock()
					votes++
					voteMutex.Unlock()
					break
				}
				time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond) // Random backoff
			}

			if err != nil {
				fmt.Printf("[Node %s] Failed to get vote from %s\n", n.ID, followerID)
			}
		}(followerID, followerURL)
	}

	// Wait for election timeout
	<-ctx.Done()

	// Check if election succeeded
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	if n.Role != CandidateRole {
		// Node might have stepped down to Follower
		fmt.Printf("[Node %s] Election aborted, role changed to %s\n", n.ID, n.Role)
		return false
	}

	if votes > len(n.Followers)/2 {
		fmt.Printf("[Node %s] Won election, becoming LEADER for term %d\n", n.ID, currentTerm)
		n.Role = LeaderRole
		n.LeaderID = n.ID
		go n.StartHeartbeat()
		return true
	}

	fmt.Printf("[Node %s] Election failed, reverting to FOLLOWER\n", n.ID)
	n.Role = FollowerRole
	return false
}

// RequestVote sends a vote request to a follower with WAL details.
func (n *Node) RequestVote(url string) (bool, error) {
	currentIndex, currentTerm := n.WAL.GetLastLogIndexAndTerm()
	votePayload := map[string]interface{}{
		"candidateID":  n.ID,
		"term":         n.Term,
		"lastLogIndex": currentIndex,
		"lastLogTerm":  currentTerm,
	}

	responseBody, err := n.Communicator.PostHTTPRequest(fmt.Sprintf("%s/vote", url), votePayload)
	if err != nil {
		return false, fmt.Errorf("failed to send vote request to %s: %w", url, err)
	}

	var response struct {
		VoteGranted bool `json:"voteGranted"`
		Term        int  `json:"term"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return false, fmt.Errorf("failed to parse vote response from %s: %w", url, err)
	}

	// Update the node's term if the follower's term is higher
	if response.Term > n.Term {
		n.Mutex.Lock()
		n.Term = response.Term
		n.Mutex.Unlock()
	}

	return response.VoteGranted, nil
}

func (n *Node) ResetElectionTimer() {
	if n.ElectionTimer != nil {
		n.ElectionTimer.Stop()
	}
	duration := time.Duration(150+rand.Intn(150)) * time.Millisecond
	n.ElectionTimer = time.AfterFunc(duration, func() {
		n.startElection()
	})
}

// MonitorLeader continuously checks the leader's status based on heartbeats
func (n *Node) MonitorLeader() {
	for {
		timeDifference := time.Since(n.LastHeartbeat)
		log.Printf("Follower %s: Time since last heartbeat: %v\n", n.ID, timeDifference)

		// Check if the leader is inactive
		if timeDifference > n.HeartbeatTimeout {
			log.Printf("Follower %s: Leader is inactive! Last heartbeat was at %v", n.ID, n.LastHeartbeat)

			// Step 1: Transition to Candidate state
			n.Mutex.Lock()
			if n.Role != FollowerRole {
				n.Mutex.Unlock()
				return // If already in Candidate or Leader state, skip
			}
			n.Role = CandidateRole
			n.Term++ // Increment term for the new election
			n.VotedFor = n.ID
			n.Mutex.Unlock()

			log.Printf("Follower %s: Transitioning to Candidate, starting election for term %d", n.ID, n.Term)

			// Step 2: Start election process
			wonElection := n.startElection()

			// Step 3: Handle Election Result
			n.Mutex.Lock()
			if wonElection {
				log.Printf("Node %s: Won the election and became the LEADER for term %d", n.ID, n.Term)
				n.Role = LeaderRole
				n.LeaderID = n.ID
				n.Mutex.Unlock()

				// Start leader-specific tasks
				go n.StartHeartbeat()
			} else {
				log.Printf("Node %s: Election failed or another leader was elected. Reverting to FOLLOWER.", n.ID)
				n.Role = FollowerRole
				n.Mutex.Unlock()
			}
		}

		time.Sleep(500 * time.Millisecond) // Check every 500ms
	}
}

// HandleReplication processes the replication data on the follower node.
func (n *Node) ProcessReplication(leaderID string, term int, key string, value []byte, timestamp int64, operation Operation) error {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()

	if term < n.Term {
		return fmt.Errorf("replication rejected due to stale term: %d", term)
	}

	if term > n.Term {
		n.Term = term
		n.LeaderID = leaderID
	}
	// Write the change to WAL
	// Apply the change to the in-memory store
	lsn, shouldReturn, returnValue := n.replicateEntry(operation, key, value)
	if shouldReturn {
		return returnValue
	}

	// Optionally, log for debugging
	fmt.Printf("Replication applied: operation=%s, key=%s, lsn=%d, term=%d\n", operation, key, lsn, term)

	return nil
}

func (n *Node) replicateEntry(operation Operation, key string, value []byte) (int64, bool, error) {
	var lsn int64
	var err error
	if operation == PUT {
		lsn, err = n.WAL.WriteEntry(int64(n.Term), "put", key, value)
	} else if operation == DELETE {
		lsn, err = n.WAL.WriteEntry(int64(n.Term), "delete", key, nil)
	} else {
		return 0, true, fmt.Errorf("unsupported operation: %s", operation)
	}

	if err != nil {
		return 0, true, fmt.Errorf("failed to write to WAL: %w", err)
	}

	if operation == PUT {
		if err := n.Store.Put(key, value); err != nil {
			return 0, true, fmt.Errorf("failed to apply replication to store: %w", err)
		}
	} else if operation == DELETE {
		if err := n.Store.Delete(key); err != nil {
			return 0, true, fmt.Errorf("failed to apply delete operation to store: %w", err)
		}
	}
	return lsn, false, nil
}

// HandleBatchReplication processes batch replication requests from the leader.
func (n *Node) ProcessBatchReplication(leaderID string, term int, entries map[string][]byte) error {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()

	if term < n.Term {
		return fmt.Errorf("outdated term in batch replication: %d < %d", term, n.Term)
	}

	// Update the term and set the leader if valid
	if term > n.Term {
		n.Term = term
		n.LeaderID = leaderID
	}

	// Append entries to WAL and apply to store
	for key, value := range entries {

		lsn, shouldReturn, returnValue := n.replicateEntry(PUT, key, value)
		if shouldReturn {
			return returnValue
		}
		// Optionally, log for debugging
		fmt.Printf("Batch replication applied: key=%s, lsn=%d, term=%d\n", key, lsn, term)
	}

	return nil
}
