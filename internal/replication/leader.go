package replication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"keyvalue-store/internal/network"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type LeaderServer struct {
	LeaderElection *RaftLeaderElection
	port           string
	httpServer     *http.Server
	routeServer    *network.Server
}

// RaftLeaderElection defines the Raft leader election and heartbeat process
type RaftLeaderElection struct {
	leaderID          string
	candidateID       string
	followerURLs      []string
	heartbeatInterval time.Duration
	heartbeatTimeout  time.Duration
	lastHeartbeat     time.Time
	term              int // Raft term for leadership
	mutex             sync.Mutex
}

// NewRaftLeaderElection creates and initializes a new Raft leader election
func NewRaftLeaderElection(candidateID string, heartbeatInterval, heartbeatTimeout time.Duration) *RaftLeaderElection {
	return &RaftLeaderElection{
		candidateID:       candidateID,
		heartbeatInterval: heartbeatInterval,
		heartbeatTimeout:  heartbeatTimeout,
		followerURLs:      make([]string, 0),
		term:              0,
	}
}

// NewLeaderServer creates and initializes a new leader server
func NewLeaderServer(candidateID, port string, heartbeatInterval, heartbeatTimeout time.Duration) (*LeaderServer, error) {
	leaderElection := NewRaftLeaderElection(candidateID, heartbeatInterval, heartbeatTimeout)
	server := &LeaderServer{
		LeaderElection: leaderElection,
		port:           port,
		httpServer:     &http.Server{Addr: port},
	}
	return server, nil
}

// StartLeaderHTTPServer starts the HTTP server for leader operations
// StartLeaderHTTPServer starts the HTTP server for the leader
func (le *LeaderServer) StartLeaderHTTPServer() {
	http.HandleFunc("/heartbeat", le.HandleHeartbeat)
	http.HandleFunc("/register_follower", le.HandleRegisterFollower)
	http.HandleFunc("/update_leader", le.HandleUpdateLeader)

	le.routeServer.RegisterRoutes()

	le.httpServer = &http.Server{Addr: le.port}

	log.Printf("Leader Server on port %s is starting...\n", le.port)

	// Create a channel to listen for OS signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Run the server in a separate goroutine
	go func() {
		if err := le.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start leader server: %v", err)
		}
	}()

	// Wait for a termination signal
	<-stopChan
	log.Println("Shutting down leader HTTP server gracefully...")

	// Gracefully shutdown the server with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := le.httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Leader HTTP server forced to shutdown: %v", err)
	}

	log.Println("Leader HTTP server shut down successfully.")
}

// HandleHeartbeat handles heartbeats from followers
func (le *LeaderServer) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	log.Println("hearbet request received")
	//le.LeaderElection.Heartbeat()
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Leader %s is alive in term %d", le.LeaderElection.leaderID, le.LeaderElection.term)
}

// HandleRegisterFollower handles registration of new followers
func (le *LeaderServer) HandleRegisterFollower(w http.ResponseWriter, r *http.Request) {
	followerURL := r.URL.Query().Get("url")
	if followerURL == "" {
		http.Error(w, "Missing follower URL", http.StatusBadRequest)
		return
	}
	le.LeaderElection.RegisterFollower(followerURL)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Follower %s registered successfully", followerURL)
}

// HandleUpdateLeader updates followers with the current leader
func (le *LeaderServer) HandleUpdateLeader(w http.ResponseWriter, r *http.Request) {
	var data struct {
		LeaderID string `json:"leaderId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	le.LeaderElection.UpdateLeader(data.LeaderID)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Leader updated to %s", data.LeaderID)
}

// StartLeaderElection starts the election process and sends heartbeats if elected as leader
func (le *RaftLeaderElection) StartLeaderElection() {
	le.mutex.Lock()
	defer le.mutex.Unlock()

	if le.leaderID == "" || le.candidateID < le.leaderID {
		le.leaderID = le.candidateID
		le.term++
		log.Printf("Node %s elected as the new leader in term %d.\n", le.leaderID, le.term)
		go le.notifyFollowers(le.followerURLs)
		le.startHeartbeatRoutine()
	}
}

// Heartbeat updates the last heartbeat and notifies followers
func (le *RaftLeaderElection) Heartbeat() {
	log.Printf("Leader %s sent heartbeat in term %d.", le.leaderID, le.term)
	le.mutex.Lock()
	le.lastHeartbeat = time.Now()
	le.mutex.Unlock()
	log.Printf("Leader %s sent heartbeat in term %d.", le.leaderID, le.term)
}

// RegisterFollower adds a new follower URL
func (le *RaftLeaderElection) RegisterFollower(followerURL string) {
	le.mutex.Lock()
	defer le.mutex.Unlock()
	le.followerURLs = append(le.followerURLs, followerURL)
	log.Printf("Follower %s registered.", followerURL)
}

func (le *RaftLeaderElection) notifyFollowers(followerURLs []string) {
	for _, url := range followerURLs {
		go func(followerURL string) {
			body, _ := json.Marshal(map[string]string{"leaderId": le.leaderID})
			resp, err := http.Post(followerURL+"/update_leader", "application/json", bytes.NewReader(body))
			if err != nil || resp.StatusCode != http.StatusOK {
				log.Printf("Failed to notify follower %s: %v", followerURL, err)
			} else {
				log.Printf("Follower %s notified of new leader %s", followerURL, le.leaderID)
			}
		}(url)
	}
}

func (le *RaftLeaderElection) startHeartbeatRoutine() {
	ticker := time.NewTicker(le.heartbeatInterval)
	defer ticker.Stop()

	for range ticker.C {
		le.Heartbeat()
	}
}

func (le *RaftLeaderElection) UpdateLeader(newLeaderID string) {
	le.mutex.Lock()
	defer le.mutex.Unlock()
	le.leaderID = newLeaderID
	log.Printf("Leader updated to %s in term %d.", newLeaderID, le.term)
}

func (le *LeaderServer) Stop() error {
	log.Println("Shutting down the leader server...")
	return le.httpServer.Close()
}
