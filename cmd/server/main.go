package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"keyvalue-store/internal/network"
	"keyvalue-store/internal/node"
	"keyvalue-store/internal/storage"
	"keyvalue-store/internal/transaction"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)
# Start the Leader in the background
echo "Starting Leader on port $LEADER_PORT..."
./keyvalue-store -role= -leaderPort=$LEADER_PORT -leaderHeartbeatInterval=$HEARTBEAT_INTERVAL -leaderHeartbeatTimeout=$HEARTBEAT_TIMEOUT &
LEADER_PID=$!
func main() {
	// Command-line flags
	//Ports are here for testing, this wil be different IP address in a cluster
	nodeRole := flag.String("role", "follower", "Role of the node: leader or follower")
	port := flag.String("port", ":8080", "Port for the node server")
	leaderURL := flag.String("leaderURL", "http://localhost:8080", "URL of the leader node (if follower)")
	heartbeatInterval := flag.Duration("heartbeatInterval", 2*time.Second, "Interval between heartbeats")
	heartbeatTimeout := flag.Duration("heartbeatTimeout", 5*time.Second, "Timeout for heartbeats")
	walPath := "wal.log"             // Write-ahead log file path
	filePath := "data.log"           // Data file path
	bufferSize := 1024               // 1KB buffer
	flushInterval := 2 * time.Second // Flush every 2 seconds

	flag.Parse()

	// Validate node role
	if *nodeRole != "leader" && *nodeRole != "follower" {
		log.Fatalf("Invalid role: %s. Must be 'leader' or 'follower'", *nodeRole)
	}

	// Initialize storage
	store, err := storage.Open(filePath)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// Initialize WAL
	wal, err := transaction.NewWAL(walPath, bufferSize, flushInterval)
	if err != nil {
		log.Fatalf("Failed to initialize WAL: %v", err)
	}
	defer wal.Close()

	// Generate a unique node ID
	nodeID := generateNodeID()
	log.Printf("Node ID: %s", nodeID)

	// Initialize the Node
	node, err := node.NewNode(
		nodeID,
		*port,
		node.NodeRole(*nodeRole),
		*leaderURL,
		store,
		wal,
		*heartbeatInterval,
		*heartbeatTimeout,
	)
	if err != nil {
		log.Fatalf("Failed to initialize node: %v", err)
	}

	// Create the server for the node
	server := network.NewServer(node, *port)
	if server == nil {
		log.Fatalf("Failed to initialize server")
	}

	// Start the server
	go func() {
		if err := server.Start(); err != nil {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("Received termination signal, shutting down...")

	node.Shutdown()
}

// Generate a random Node ID
func generateNodeID() string {
	randomBytes := make([]byte, 16) // 16 bytes = 128 bits
	if _, err := rand.Read(randomBytes); err != nil {
		log.Fatalf("Failed to generate random Node ID: %v", err)
	}
	return hex.EncodeToString(randomBytes)
}
