package replication

import (
	"log"
	"sync"
	"time"
)

// FailoverManager manages the process of detecting leader failure and triggering failover
type FailoverManager struct {
	leaderElection   *RaftLeaderElection
	triggered        bool          // To prevent multiple failover triggers
	heartbeatTimeout time.Duration // Timeout for leader's heartbeat
	mutex            sync.Mutex
}

// NewFailoverManager creates and initializes a new FailoverManager
func NewFailoverManager(leaderElection *RaftLeaderElection, heartbeatTimeout time.Duration) *FailoverManager {
	return &FailoverManager{
		leaderElection:   leaderElection,
		triggered:        false,
		heartbeatTimeout: heartbeatTimeout,
	}
}

// TriggerFailover initiates the failover process by starting a new leader election
func (fm *FailoverManager) TriggerFailover() {
	fm.mutex.Lock()
	defer fm.mutex.Unlock()

	if fm.triggered {
		log.Println("Failover already in progress, skipping re-trigger.")
		return
	}

	fm.triggered = true
	log.Println("Triggering failover process...")

	// Start leader re-election process
	fm.leaderElection.StartLeaderElection()
	log.Println("Failover process complete. New leader elected.")

	// Reset failover triggered state after election
	fm.triggered = false
}

// MonitorLeaderFailure continuously checks if the leader is inactive and triggers failover
func (fm *FailoverManager) MonitorLeaderFailure() {
	for {
		fm.leaderElection.mutex.Lock()
		if time.Since(fm.leaderElection.lastHeartbeat) > fm.heartbeatTimeout {
			log.Println("Leader heartbeat timeout detected. Initiating failover...")
			fm.TriggerFailover()
		}
		fm.leaderElection.mutex.Unlock()
		time.Sleep(1 * time.Second)
	}
}

// StartFailover starts monitoring for leader failure
func (fm *FailoverManager) StartFailover() {
	go fm.MonitorLeaderFailure()
}
