package node

import "fmt"

func (n *Node) HandleWrite(key string, value []byte) error {
	if n.Role != LeaderRole {
		return fmt.Errorf("Node %s is not a Leader", n.ID)
	}

	if key == "" {
		return fmt.Errorf("Key %s is not valid", key)
	}

	// Step 1: Write to WAL
	lsn, err := n.WAL.WriteEntry(int64(n.Term), "PUT", key, value)
	if err != nil {
		return fmt.Errorf("failed to write to WAL: %v", err)
	}

	// Write to storage
	err = n.Store.Put(key, value)
	if err != nil {
		return err
	}

	// Replicate to Followers
	for _, followerURL := range n.Followers {
		go func(url string) {
			err := n.ReplicateToFollower(url, key, value)
			if err != nil {
				fmt.Printf("Failed to replicate to follower: %s\n", url)
			}
		}(followerURL)
	}
	// Step 3: Update LastAppliedOffset and persist checkpoint
	n.LastAppliedOffset = lsn
	if err := n.SaveCheckpoint(); err != nil {
		return fmt.Errorf("failed to save checkpoint after write: %v", err)
	}
	return nil
}

func (n *Node) HandleGet(key string) ([]byte, error) {
	// Only allow read operations on Leader or Follower
	if n.Role != LeaderRole && n.Role != FollowerRole {
		return nil, fmt.Errorf("Node %s is not ready to serve reads", n.ID)
	}

	// Fetch from storage
	value, err := n.Store.Get(key)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve key %s: %v", key, err)
	}

	return value, nil
}

func (n *Node) HandleDelete(key string) error {
	if n.Role != LeaderRole {
		return fmt.Errorf("Node %s is not a Leader", n.ID)
	}

	// Write DELETE to WAL
	lsn, err := n.WAL.WriteEntry(int64(n.Term), "DELETE", key, nil)
	if err != nil {
		return fmt.Errorf("failed to write DELETE to WAL: %v", err)
	}

	// Apply DELETE to storage
	err = n.Store.Delete(key)
	if err != nil {
		return fmt.Errorf("failed to delete key from storage: %v", err)
	}

	// Replicate DELETE to followers
	for _, followerURL := range n.Followers {
		go func(url string) {
			err := n.ReplicateToFollower(url, key, nil)
			if err != nil {
				fmt.Printf("Failed to replicate DELETE to follower: %s\n", url)
			}
		}(followerURL)
	}

	// Update LastAppliedOffset and save checkpoint
	n.LastAppliedOffset = lsn
	if err := n.SaveCheckpoint(); err != nil {
		return fmt.Errorf("failed to save checkpoint after DELETE: %v", err)
	}

	return nil
}

func (n *Node) HandleBatchPut(entries map[string][]byte) error {
	if n.Role != LeaderRole {
		return fmt.Errorf("Node %s is not a Leader", n.ID)
	}

	// Step 1: Write Batch to WAL
	for key, value := range entries {
		lsn, err := n.WAL.WriteEntry(int64(n.Term), "PUT", key, value)
		if err != nil {
			return fmt.Errorf("failed to write batch PUT to WAL: %v", err)
		}
		n.LastAppliedOffset = lsn
	}

	// Step 2: Write Batch to Storage
	err := n.Store.BatchPut(entries)
	if err != nil {
		return fmt.Errorf("failed to write batch PUT to storage: %v", err)
	}

	pool := make(chan struct{}, 10) // Limit concurrency to 10

	for _, followerURL := range n.Followers {
		go func(url string) {
			pool <- struct{}{}        // Acquire a slot
			defer func() { <-pool }() // Release the slot

			err := n.BatchReplicateToFollower(url, entries)
			if err != nil {
				fmt.Printf("Failed to replicate batch to follower: %s\n", url)
			}
		}(followerURL)
	}

	// Wait for all operations to complete
	for i := 0; i < cap(pool); i++ {
		pool <- struct{}{}
	}

	// Step 4: Save Checkpoint
	if err := n.SaveCheckpoint(); err != nil {
		return fmt.Errorf("failed to save checkpoint after batch write: %v", err)
	}

	return nil
}

func (n *Node) HandleQueryBatch(startKey, endKey string) (map[string][]byte, error) {
	if n.Role != LeaderRole && n.Role != FollowerRole {
		return nil, fmt.Errorf("Node %s is not ready to serve range queries", n.ID)
	}

	// Use Storage's RangeQuery
	results, err := n.Store.RangeQuery(startKey, endKey)
	if err != nil {
		return nil, fmt.Errorf("failed to perform range query: %v", err)
	}

	return results, nil
}
