package main

import (
	"flag"
	"keyvalue-store/internal/replication"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// Define command-line flags for leader and follower
	isLeader := flag.Bool("leader", false, "Start the server as a leader")
	isFollower := flag.Bool("follower", false, "Start the server as a follower")
	leaderPort := flag.String("leaderPort", ":8080", "Port for the leader server")
	followerPort := flag.String("followerPort", ":8081", "Port for the follower server")
	leaderHeartbeatInterval := flag.Duration("leaderHeartbeatInterval", 2*time.Second, "Interval between leader heartbeats")
	leaderHeartbeatTimeout := flag.Duration("leaderHeartbeatTimeout", 5*time.Second, "Timeout for leader heartbeats")
	leaderURL := flag.String("leaderURL", "http://localhost:8080", "URL of the leader server (for follower)")

	// Parse the command-line flags
	flag.Parse()

	// Ensure that either the leader or follower flag is provided, but not both
	if *isLeader && *isFollower {
		log.Fatal("Cannot start both a leader and a follower at the same time.")
	}
	if !*isLeader && !*isFollower {
		log.Fatal("Please specify either -leader or -follower.")
	}

	// Handle leader case
	if *isLeader {
		// Check if the leader is already running (this can be done via a file lock or network check)
		// For simplicity, let's create a lock file (you can replace it with a more robust check)
		lockFile := "/tmp/leader.lock"
		if _, err := os.Stat(lockFile); err == nil {
			log.Fatal("A leader is already running.")
		}

		// Create a lock file to prevent multiple leader instances
		file, err := os.Create(lockFile)
		if err != nil {
			log.Fatalf("Error creating lock file: %v", err)
		}
		defer file.Close()

		// Set up cleanup to remove the lock file on program exit
		defer func() {
			log.Println("Removing lock file...")
			err := os.Remove(lockFile)
			if err != nil {
				log.Printf("Failed to remove lock file: %v", err)
			} else {
				log.Println("Lock file removed successfully.")
			}
		}()

		//
		// Generate a unique candidate ID for the leader
		leaderCandidateID := "leader-instance-1" // Replace this with your unique identifier logic

		// Leader configuration with the new signature
		leaderServer, err := replication.NewLeaderServer(
			leaderCandidateID,
			*leaderPort,
			*leaderHeartbeatInterval,
			*leaderHeartbeatTimeout,
		)

		// Leader configuration
		//leaderServer, err := replication.NewLeaderServer(*leaderHeartbeatInterval, *leaderHeartbeatTimeout, *leaderPort)
		if err != nil {
			log.Fatalf("Failed to create leader server: %v", err)
		}

		// Start leader server and send heartbeats
		go leaderServer.StartLeaderHTTPServer()
		go leaderServer.LeaderElection.StartLeaderElection()

		log.Printf("Leader server started on %s...\n", *leaderPort)

		// Wait for termination signal (Ctrl+C or other signals)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Block until we receive a signal
		<-sigChan
		log.Println("Received termination signal, shutting down...")

		// Gracefully shutdown (e.g., stop the server, cleanup, etc.)
		return
	}

	// Handle follower case
	if *isFollower {
		// Follower configuration with the failover manager (we will need a SimpleLeaderElection for failover)

		// Generate a unique candidate ID for the leader
		leaderCandidateID := "leader-instance-1" // Replace this with your unique identifier logic

		// Updated leader election initialization with candidate ID
		leaderElection := replication.NewRaftLeaderElection(leaderCandidateID, *leaderHeartbeatInterval, *leaderHeartbeatTimeout)

		// Initialize the failover manager with the updated leader election instance
		failoverManager := replication.NewFailoverManager(leaderElection, *leaderHeartbeatTimeout)

		// Initialize follower with port, leader URL, and failover manager
		follower := replication.NewFollower(*followerPort, *leaderURL, failoverManager)

		// Start follower server
		go func() {
			if err := follower.StartFollowerHTTPServer(); err != nil {
				log.Fatalf("Follower server failed: %v", err)
			}
		}()

		// Start monitoring leader's heartbeat
		go follower.MonitorLeader()

		log.Printf("Follower server started on %s, monitoring leader at %s...\n", *followerPort, *leaderURL)

		// Wait for termination signal (Ctrl+C or other signals)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

		// Block until we receive a signal
		<-sigChan
		log.Println("Received termination signal, shutting down...")
	}
}
