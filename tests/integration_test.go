package tests

import (
	"keyvalue-store/internal/replication"
	"log"
	"testing"
	"time"
)

func TestLeaderFollowerInteraction(t *testing.T) {
	// Leader configuration
	candidateID := "test-leader"
	port := ":8080"
	heartbeatInterval := 2 * time.Second
	heartbeatTimeout := 5 * time.Second

	// Create and initialize the leader server
	leaderServer, err := replication.NewLeaderServer(candidateID, port, heartbeatInterval, heartbeatTimeout)
	if err != nil {
		t.Fatalf("Failed to create leader server: %v", err)
	}

	// Initialize the failover manager for leader election
	failoverManager := replication.NewFailoverManager(leaderServer.LeaderElection, heartbeatTimeout)

	// Start leader server in a separate goroutine
	go func() {
		leaderServer.StartLeaderHTTPServer()
	}()

	// Allow leader server to start up
	time.Sleep(1 * time.Second)

	// Follower configuration with failover manager passed
	follower := replication.NewFollower(":8081", "http://localhost"+port, failoverManager)

	// Start follower HTTP server in a separate goroutine
	go func() {
		if err := follower.StartFollowerHTTPServer(); err != nil {
			log.Fatalf("Follower server failed: %v", err)
		}
	}()

	// Allow follower server to start up
	time.Sleep(1 * time.Second)

	// Start monitoring leader heartbeat in a separate goroutine
	go follower.MonitorLeader()

	// Allow time for heartbeats
	time.Sleep(5 * time.Second)

	// Verify that the leader is considered active by the follower
	if !follower.IsLeaderActive() {
		t.Fatalf("Follower incorrectly marked leader as inactive")
	}

	// Simulate leader failure
	leaderServer.Stop() // This should stop the leader server

	// Allow some time for failover detection
	time.Sleep(heartbeatTimeout + time.Second)

	// Verify that the follower detects leader failure and initiates election
	if follower.IsLeaderActive() {
		t.Fatalf("Follower failed to detect leader failure")
	}

	// Verify that the failover manager initiated a new leader election
	// This might be verified by checking the logs or using mock functions if needed
}
