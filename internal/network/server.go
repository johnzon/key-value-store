package network

import (
	"keyvalue-store/internal/node"
	"log"
	"net/http"
)

// Server represents the network layer for handling API requests
type Server struct {
	Node         node.NodeHandler      // Reference to the associated Node
	HTTPServer   *http.Server          // Embedded HTTP server
	Router       *http.ServeMux        // Router for API endpoints
	Communicator node.NodeCommunicator // Handles inter-node communication
}

func NewServer(node node.NodeHandler, port string) *Server {
	router := http.NewServeMux()
	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	s := &Server{
		Node:       node,
		HTTPServer: server,
		Router:     router,
	}

	// Define API routes
	s.Router.HandleFunc("/read", s.HandleRead)
	s.Router.HandleFunc("/put", s.HandleWrite)
	s.Router.HandleFunc("/batchput", s.HandleBatchWrite)
	s.Router.HandleFunc("/delete", s.HandleDelete)
	s.Router.HandleFunc("/range", s.HandleBatchQuery)
	s.Router.HandleFunc("/heartbeat", s.handleHeartbeat)
	s.Router.HandleFunc("/register_follower", s.HandleRegisterFollower)
	s.Router.HandleFunc("/vote", s.handleVoteRequest)
	s.Router.HandleFunc("/batch-replicate", s.HandleBatchReplicate)
	s.Router.HandleFunc("/replicate", s.HandleReplication)

	return s
}

// Start runs the server based on the node's role
func (s *Server) Start() error {
	serverReady := make(chan bool)

	// Start the HTTP server in a goroutine
	go func() {
		log.Printf("Server starting on port %s...", s.HTTPServer.Addr)
		serverReady <- true // Signal server readiness
		if err := s.HTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for server readiness
	<-serverReady

	// Start Node-specific logic

	if s.Node.IsLeader() {
		log.Println("Starting node as LEADER...")
		go s.Node.StartLeader()
	} else if s.Node.IsFollower() {
		log.Println("Starting node as FOLLOWER...")
		go s.Node.StartFollower()
	} else {
		log.Fatalf("Invalid node role specified")
	}

	return nil
}
