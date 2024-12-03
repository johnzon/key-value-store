package replication

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Follower represents the follower's state in the leader election process
type Follower struct {
	followerID       string
	leaderURL        string
	isLeaderActive   bool
	lastHeartbeat    time.Time
	heartbeatTimeout time.Duration
	mutex            sync.Mutex
	serverPort       string
	server           *http.Server     // HTTP server instance
	failoverManager  *FailoverManager // Add failover manager
	leaderTerm       int              // Raft term of the leader
}

// NewFollower creates and initializes a new Follower instance with the failover manager
func NewFollower(port, leaderURL string, failoverManager *FailoverManager) *Follower {
	return &Follower{
		followerID:       fmt.Sprintf("follower-%d", time.Now().UnixNano()),
		leaderURL:        leaderURL,
		isLeaderActive:   true,
		lastHeartbeat:    time.Now(),
		heartbeatTimeout: 10 * time.Second, // Default timeout for simplicity
		serverPort:       port,
		failoverManager:  failoverManager, // Initialize failover manager
		leaderTerm:       0,               // Initialize leader's term
	}
}

// StartFollowerHTTPServer starts an HTTP server to receive leader heartbeats
func (f *Follower) StartFollowerHTTPServer() error {
	http.HandleFunc("/heartbeat", f.HandleHeartbeat)

	f.server = &http.Server{Addr: f.serverPort}

	log.Printf("Follower %s is listening on %s for leader heartbeats...\n", f.followerID, f.serverPort)

	f.lastHeartbeat = time.Now()

	err := f.RegisterWithLeader()
	if err != nil {
		return err
	}

	// Create a channel to listen for OS signals
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	// Run the server in a separate goroutine
	go func() {
		if err := f.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Follower HTTP server failed: %v", err)
		}
	}()

	// Wait for a termination signal
	<-stopChan
	log.Println("Shutting down follower HTTP server gracefully...")

	// Gracefully shutdown the server with a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := f.server.Shutdown(ctx); err != nil {
		log.Fatalf("Follower HTTP server forced to shutdown: %v", err)
	}

	log.Println("Follower HTTP server shut down successfully.")
	return nil
}

// // StartFollowerHTTPServer starts an HTTP server to receive leader heartbeats
// func (f *Follower) StartFollowerHTTPServer() error {
// 	http.HandleFunc("/heartbeat", f.HandleHeartbeat)

// 	f.server = &http.Server{Addr: f.serverPort}

// 	log.Printf("Follower %s is listening on %s for leader heartbeats...\n", f.followerID, f.serverPort)

// 	f.lastHeartbeat = time.Now()

// 	err := f.RegisterWithLeader()
// 	if err != nil {
// 		return err
// 	}

// 	go f.MonitorLeader()
// 	return f.server.ListenAndServe()
// }

// HandleHeartbeat updates the last received heartbeat time and checks leader term
func (f *Follower) HandleHeartbeat(w http.ResponseWriter, r *http.Request) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.lastHeartbeat = time.Now()
	f.isLeaderActive = true
	log.Printf("Follower %s received heartbeat from leader at %v", f.followerID, f.lastHeartbeat)
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Heartbeat received by follower %s", f.followerID)
}

// MonitorLeader continuously checks the leader's status based on heartbeats
func (f *Follower) MonitorLeader() {
	for {
		f.mutex.Lock()
		timeDifference := time.Since(f.lastHeartbeat)
		log.Printf("Follower %s: Time since last heartbeat: %v\n", f.followerID, timeDifference)

		if time.Since(f.lastHeartbeat) > f.heartbeatTimeout {
			if f.isLeaderActive {
				log.Printf("Follower %s: Leader is inactive!\n", f.followerID)
				log.Printf("Follower %s: Leader is inactive! Last heartbeat was at %v, current time: %v", f.followerID, f.lastHeartbeat, time.Now())
				f.isLeaderActive = false

				// Trigger failover via the FailoverManager
				if f.failoverManager != nil {
					f.failoverManager.TriggerFailover()
				}
			}
		}
		f.mutex.Unlock()

		time.Sleep(time.Second) // Check every second
	}
}

// IsLeaderActive returns whether the leader is currently considered active
func (f *Follower) IsLeaderActive() bool {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	return f.isLeaderActive
}

// getFollowerURL generates the follower's URL based on the server port
func (f *Follower) getFollowerURL() string {
	return "http://localhost" + f.serverPort
}

func (f *Follower) RegisterWithLeader() error {
	resp, err := http.Get(f.leaderURL + "/register_follower?url=" + f.getFollowerURL())
	if err != nil {
		return fmt.Errorf("error registering follower with leader: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to register follower, status code: %d", resp.StatusCode)
	}

	log.Printf("Follower %s successfully registered with leader %s", f.followerID, f.leaderURL)
	return nil
}
