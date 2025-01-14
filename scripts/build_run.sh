#!/bin/bash

# Define ports for leader and followers
LEADER_PORT=:8080
FOLLOWER_PORT_1=:8081
FOLLOWER_PORT_2=:8082
LEADER_ROLE=:"Leader"
FOLLOWER_ROLE=:"Follower"

# Define the leader's heartbeat interval and timeout
HEARTBEAT_INTERVAL="2s"
HEARTBEAT_TIMEOUT="10s"

# Define the leader URL for followers to connect to
LEADER_URL="http://localhost:8080"

# Build the application
echo "Building the application..."
go build -o keyvalue-store cmd/server/main.go
if [ $? -ne 0 ]; then
    echo "Build failed. Exiting."
    exit 1
fi

# Start the Leader in the background
echo "Starting Leader on port $LEADER_PORT..."
./keyvalue-store -role=$LEADER_ROLE -leaderURL=$LEADER_URL -leaderPort=$LEADER_PORT -leaderHeartbeatInterval=$HEARTBEAT_INTERVAL -leaderHeartbeatTimeout=$HEARTBEAT_TIMEOUT &
LEADER_PID=$!

# Wait a bit to ensure leader starts
sleep 2

# Start the first Follower in the background
echo "Starting Follower 1 on port $FOLLOWER_PORT_1, connecting to leader at $LEADER_URL..."
./keyvalue-store -role=$LEADER_ROLE -leaderURL=$LEADER_URL  &
FOLLOWER_PID_1=$!

# Start the second Follower in the background
echo "Starting Follower 2 on port $FOLLOWER_PORT_2, connecting to leader at $LEADER_URL..."
./keyvalue-store -role=$LEADER_ROLE -leaderURL=$LEADER_URL  &
FOLLOWER_PID_2=$!

# Wait for all processes to finish (or run indefinitely)
wait $LEADER_PID
wait $FOLLOWER_PID_1
wait $FOLLOWER_PID_2

echo "Run complete."
