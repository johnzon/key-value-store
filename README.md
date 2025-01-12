# Key/Value  Store System Documentation

## Objective
To design and implement a network-available, persistent Key/Value store that adheres to the provided interface and achieves specific performance, reliability, and scalability requirements. The system is implemented in golang and uses only the standard library.

---

## Requirements

### Functional Requirements 
the system should have the following interfaces
1. **Put(Key, Value)**: Inserts or updates the value for the given key.
2. **Read(Key)**: Retrieves the value associated with the given key.
3. **ReadKeyRange(StartKey, EndKey)**: Fetches values for keys within the specified range.
4. **BatchPut(..keys, ..values)**: Allows multiple key-value pairs to be written in a single operation.
5. **Delete(Key)**: Removes the specified key and its associated value.

### Non-Functional Requirements 
1. **Low latency**
2. **High throughput**
3. **Crash Friendliness**
4. **Replication(Nice to have)**
5. **Resilience**
---

## Key Design Decisions

### 1. Low Latency per Item Read or Written
- **In-Memory Index (`map[string]int64`)**: Enables O(1) key-to-offset mapping for fast lookups.
- **Secondary Sorted Index (`btree.BTree`)**: Supports efficient range queries, minimizing latency for sequential reads.
- **Thread-Safe Writes with `sync.Mutex`**: Ensures consistent updates while minimizing latency for concurrent operations.

### 2. High Throughput, Especially When Writing an Incoming Stream of Random Items
- **Append-Only Write-Ahead Log (WAL)**: Sequential writes minimize disk seek times, enhancing write throughput.
- **Batch Operations**: Allows multiple writes to be processed together, reducing the overhead associated with individual writes.


### 3. Ability to Handle Datasets Much Larger Than RAM Without Degradation
- **Occasional Flushing of Index to Disk**: Large in-memory index structures are occasionally flushed to disk to reduce memory usage and ensure consistency without impacting write performance.
- **Checkpointing for Faster Recovery**: Periodic snapshots of the in-memory index reduce recovery time by limiting the need to process the entire WAL after a crash.


### 4. Crash Friendliness (Fast Recovery and No Data Loss)
- **WAL Recovery on Crash**: On startup, the system reads the WAL to rebuild the in-memory index, ensuring no data is lost.
- **Clean Shutdown Mechanism**: Ensures that all data is flushed and state is consistent before the system shuts down.
- **Checkpointing**: Reduces recovery time by limiting the extent of WAL replay during initialization.


### 5. Predictable Behavior Under Heavy Access Load or Large Volume
- **Replication**: Allows access from multiple nodes
- **Sharding**: While sharding can greatly help here, this implementation does not cover sharding.


### 6. Replicate Data to Multiple Nodes
- **Asynchronous Replication**: Ensures that data is propagated to replicas without impacting the performance of the primary node.
- **Design Extensibility**: The architecture allows for WAL sharing across nodes, facilitating replication.


### 7. Handle Automatic Failover to Other Nodes
- **Node Monitoring with Raft Election**: Enables nodes to monitor each other and automatically elect a new leader in case of failure.
- **Checkpointing**: Enhances failover readiness by enabling quicker recovery from a known consistent state.

### 8. Trade Offs
- **Eventual consistency**: Asynchronous replication while contributing to low latency has the side effects of eventual consistency as against strong consistency after a write operation.
- **Simplicity**: The complexity of Sharding has not been introduce yet for simplicity reason. Each Shard will have its node replicas

---
## Solution Components

### Storage Component

The storage layer uses a **log-structured design** that combines an append-only log file, an in-memory index, and a secondary B-Tree index for flexible lookups. All writes are appended to the log file, ensuring durability. The in-memory index maintains key-to-file-offset mappings for fast lookups, while the secondary B-Tree index supports range queries and additional query types. Periodically, the in-memory index and secondary index are flushed to disk to maintain persistence and crash recovery.

##### Reasons for This Design

1. **Simplicity**: This approach is straightforward to implement and avoids the complexity of full LSM Trees, such as segment compaction and merging.
2. **Write Efficiency**: The append-only log ensures fast, sequential writes, which are optimal for high-throughput scenarios.
3. **Read Efficiency**: The combination of in-memory and secondary indices allows for quick lookups and range queries without rebuilding indices.
4. **Crash Recovery**: The append-only log ensures durability, while periodic flushing of indices supports fast recovery after crashes.

##### Possible Alternatives

1. **Full LSM Tree**:
   - **Pros**: Better suited for large-scale systems with frequent writes and updates due to its efficient compaction process.
   - **Cons**: Adds complexity with compaction and merging, which may be unnecessary for simpler use cases.

2. **Hash Map-Based Storage**:
   - **Pros**: O(1) lookups and updates, ideal for low-latency requirements and single-key operations.
   - **Cons**: Inefficient for range queries and doesn’t scale well for large datasets without significant memory.

3. **Pure B-Tree Storage**:
   - **Pros**: Supports efficient range queries and maintains sorted order directly on disk.
   - **Cons**: Slower writes due to the need for balanced tree updates and higher disk I/O.

#### Why This Design is Better for This Assignment

This design strikes a balance between simplicity and functionality, making it suitable for a small-to-medium-scale key-value store. It provides efficient writes, fast lookups, and range query support without the overhead of complex compaction or balancing mechanisms. For this assignment, it ensures core objectives like low latency, high throughput, and crash recovery without introducing unnecessary complications.

### Write-Ahead Log (WAL) Component

The **Write-Ahead Log (WAL)** is crucial for durability, crash recovery, and data consistency in distributed systems. It logs all operations sequentially, enabling recovery from crashes by replaying log entries after the last consistent state.

#### Key Features
- **Durability**: Ensures data is written to disk before changes are applied.
- **Crash Recovery**: Replays operations after a crash to restore consistency.
- **Raft Compatibility**: Tracks operations with terms and LSNs, integrating with Raft consensus.

#### WAL Structure
- **WALWriter Interface**: Methods for writing logs, recovery, and managing the last log index and term.
- **WALEntry Struct**: Defines log entries with fields like LSN, term, operation, key, value, and timestamp.
- **WAL Struct**: Manages the file, buffer, periodic flushes, and recovery.

#### Methods
- **WriteEntry**: Adds an entry to the WAL and flushes if the buffer size is exceeded.
- **FlushBuffer**: Writes buffered entries to the disk, ensuring durability.
- **Recover**: Replays WAL entries from the last applied LSN to restore the system.
- **SetCommittedIndex**: Updates the committed index for the system state.

### Raft Leader Election Component

In the Raft consensus algorithm, nodes in the system transition between roles of **Leader**, **Follower**, and **Candidate** to maintain high availability and fault tolerance. The leader election process ensures that only one node acts as the leader at any given time. This process is crucial to avoid split-brain scenarios and ensure consistency across the system.
The leader election process is triggered when a node detects that the current leader is inactive or fails to send heartbeats within a specified timeout period.


1. **Follower Node Monitoring**:
   - Followers register with the Leader node as part of post startup operation. Followers continuously monitor the leader's heartbeat. If no heartbeat is received within the timeout, the follower assumes that the leader is inactive and transitions to the **Candidate** role.
   - The node increments its term and votes for itself as part of the election process.

2. **Candidate Role and Election**:
   - Once in the **Candidate** role, the node broadcasts a vote request to all other followers. Each vote request includes the candidate’s term, last log index, and term.
   - If the candidate receives a majority of votes, it becomes the leader for that term.

3. **Leader Transition**:
   - Upon winning the majority of votes, the node transitions to the **Leader** role, initiates periodic heartbeats to maintain leadership, and starts managing followers.
   - If the election fails (i.e., the candidate does not receive enough votes), the node reverts to the **Follower** role and waits for the next election cycle.

4. **Heartbeat Mechanism**:
   - The leader node sends regular heartbeats to its followers to assert its authority and prevent new elections. Heartbeats are sent concurrently to all followers via a worker pool to optimize concurrency.

5. **Follower Node Behavior**:
   - Followers receive heartbeats from the leader and update their internal state, including the leader ID and term.
   - If a follower detects a higher term from the leader, it transitions to the **Follower** role and updates its leader information.


#### Node Role Transitions

- **From Follower to Candidate**: A follower becomes a candidate if it does not receive a heartbeat from the leader within a timeout period. It then starts the election process.
- **From Candidate to Leader**: A candidate becomes the leader if it receives a majority of votes from followers.
- **From Leader to Follower**: If a leader detects a higher term from another node, it steps down and becomes a follower.

#### Justification
- **Simplicity and Understandability**: Compare to paxos and other node election algorithms, Raft is simple for our use case.

#### Current Limitation
- This implementation seeds the leader during startup since no external node ochestration is implemented yet. This implies that a node can be specified as Leader during startup. In real time, node ochestration can be delegated to an external node registry.


## Replication Strategy

The replication process is designed to ensure strong consistency and high availability in a distributed system. Our replication mechanism relies on writing to a Write-Ahead Log (WAL) and asynchronously replicating log entries to other nodes in the cluster. This approach provides an efficient, fault-tolerant method of ensuring that changes are propagated across the system while maintaining consistency.

### Workflow

1. **WAL Writing**: When a new log entry is created (e.g., a client request or state change), the entry is first written to the local WAL. This guarantees durability and ensures that changes are not lost in the event of a crash.

2. **Asynchronous Replication**: After writing to the WAL, the entry is asynchronously replicated to follower nodes. This ensures that the leader can continue processing new requests without waiting for followers to acknowledge the replication. Replication is done in parallel, leveraging concurrency to improve throughput and minimize latency.

3. **Last Committed Index**: Once the log entry is successfully replicated to the majority of followers, the leader commits the entry by appending a last committed index to the WAL. This index indicates the point at which the entry is considered durable and visible to all nodes in the system.

4. **Failure Recovery and Retry Mechanism**: In the case of a replication failure, the system implements an exponential backoff retry strategy. This ensures that replication attempts are spaced out over time, reducing the load on the network and minimizing the chances of overwhelming the system. Each failed replication is retried with an increasing delay, helping to ensure that eventual consistency is reached without overburdening the system.

5. **Replication Timeout and Consistency**: A replication timeout is set for each entry to ensure that it is eventually committed to a majority of nodes. If the replication fails within this timeout window, the leader will retry the operation using the backoff mechanism until success is achieved. This guarantees that the system maintains consistency across the cluster, even in the face of transient failures.

### Justification for Using WAL-Based Replication

- **Fault Tolerance**: By writing to a WAL before attempting replication, we ensure that the system can recover from crashes or network failures without losing data. This approach provides a durable log of all changes, which can be replayed in the event of a failure, ensuring that no committed entry is lost.

- **Asynchronous Replication for Performance**: Asynchronous replication ensures that the leader node does not become blocked while waiting for followers to acknowledge the replication. This improves the system's performance, as the leader can continue processing client requests without being slowed down by replication latency. This is critical for systems that require high throughput and low-latency processing.

- **Scalable and Efficient**: Asynchronous replication is highly scalable, as the leader can replicate log entries to multiple followers concurrently. The retry mechanism with exponential backoff further optimizes replication by reducing the load during failure scenarios, ensuring that the system can scale without unnecessary strain.

- **Consistency Guarantees**: The combination of WAL and majority-based replication ensures strong consistency. The leader guarantees that all committed entries are replicated to the majority of followers, preventing divergent state and ensuring that all nodes are eventually consistent.

---

## Why Go (Golang)?

The decision to use **Go (Golang)** for this project was based on several key factors that align with the project's requirements for **performance**, **scalability**, and **ease of development**:

1. **Concurrency Support**:
   - Go’s built-in support for concurrency through **goroutines** and **channels** makes it well-suited for building scalable, concurrent systems like key-value stores. This allows for handling multiple simultaneous read and write operations efficiently, which is crucial in a high-throughput system.

2. **Performance**:
   - Go offers **high performance** similar to lower-level languages like C, while maintaining the simplicity and readability of higher-level languages. The language's design allows for fast execution, making it ideal for latency-sensitive operations such as data reads and writes.

3. **Simplicity and Maintainability**:
   - Go’s minimalistic syntax and focus on simplicity lead to faster development cycles and easier code maintainability. The language's standard library is rich, and it provides all the essential tools for network communication, concurrency, and persistence without requiring third-party libraries.

4. **Scalability**:
   - Go is designed with **scalability** in mind, both in terms of concurrency and system architecture. Its lightweight goroutines allow for the creation of highly scalable systems that can handle large numbers of concurrent requests, making it ideal for large-scale key-value storage systems.

5. **Strong Standard Library**:
   - Go comes with an extensive **standard library** for network programming, file handling, and data persistence. This enables building robust systems with minimal external dependencies. For example, we can rely on Go's **net/http** package for building the API and **sync** for safe concurrent access to shared resources.

6. **Cross-Platform**:
   - Go is known for its ability to compile to **multiple platforms** with a single codebase. This ensures that the system can be deployed on various operating systems without requiring significant changes to the codebase.

7. **Community and Ecosystem**:
   - Go has a large, active community and a growing ecosystem of tools and libraries, which accelerates development and ensures that the language remains up-to-date with modern development practices.

Given these strengths, **Go** provides the optimal foundation for building a fast, concurrent, and reliable key-value store system.


## Project Structure

```plaintext
keyvalue-store/
├── cmd/
│   └── server/
│       └── main.go         # Entry point for the application
├── internal/
│   ├── storage/            # Storage layer
│   │   ├── storage.go      # Core storage implementation
│   ├── transaction/        # Transaction layer
│   │   ├── wal.go          # Write-ahead logging
│   ├── network/            # Networking layer
│   │   ├── server.go       # HTTP server setup
│   │   ├── handlers.go     # HTTP API handlers
│   ├── node/               # Replication layer (bonus)
│   │   ├── leader.go       # Leader logic
│   │   ├── follower.go     # Follower logic
│   │   ├── node.go         # node core logic
│   │   ├── communicator.go # nodes communication  logic
│   │   ├── nodeHanlder.go  # Node interface
│   │   └── request.go     # reuqest processing logic
├── scripts/
│   ├── build_run.sh            # Build script
├── tests/
├── Dockerfile              # Dockerfile for containerization (to be added)
├── go.mod                  # Go module definition
├── go.sum                  # Dependency tracking
└── README.md               # Project documentation
```
## How to Set Up and Run the KeyValue-Store Application

Follow the steps below to set up and run the KeyValue-Store application.

1. **Install Go**: Ensure Go is installed on your system. The version used for this project is:  version go1.18.3 darwin/arm64.
2. **Clone the Repository**: Clone the project repository and navigate to its root directory. 
3. Make the `build_run.sh` script in script folder executable: `chmod +x ./scripts/build_run.sh` .
4. **Run the script** `./scripts/build_run.sh` .

- Ensure all ports specified in the script (e.g., `8080`, `8081`, `8082`) are available on your system.
- The script assumes the Go environment is properly set up and `GOPATH` is configured if needed.

## API Endpoints for Key-Value Store

### 1. `/put` Route - Put a Key-Value Pair

- **HTTP Method**: `PUT`
- **Endpoint**: `/put?key={key}`
- **Expected Payload**: The `key` is provided as a query parameter, and the value is sent in the request body.
- **Sample Request**:
    ```bash
    curl -X PUT "http://localhost:8080/put?key=myKey" -d "myValue"
    ```

- **Success Response**:
    - **HTTP Status**: `200 OK`

- **Error Response**:
    - If the HTTP method is incorrect: `405 Method Not Allowed`
    - If the `key` parameter is missing: `400 Bad Request`
    - If there’s an issue reading the request body: `500 Internal Server Error`

---

### 2. `/read` Route - Read a Value by Key

- **HTTP Method**: `GET`
- **Endpoint**: `/read?key={key}`
- **Expected Payload**: The `key` is provided as a query parameter.
- **Sample Request**:
    ```bash
    curl "http://localhost:8080/read?key=myKey"
    ```

- **Success Response**:
    - **HTTP Status**: `200 OK`
    - **Response Body**: The stored value
    ```json
    "myValue"
    ```

- **Error Response**:
    - If the key is not found: `404 Not Found`
    - If the `key` parameter is missing: `400 Bad Request`
    - If the HTTP method is incorrect: `405 Method Not Allowed`

---

### 3. `/range` Route - Read a Range of Keys

- **HTTP Method**: `GET`
- **Endpoint**: `/range?start={startKey}&end={endKey}`
- **Expected Payload**: The `start` and `end` keys are provided as query parameters.
- **Sample Request**:
    ```bash
    curl "http://localhost:8080/range?start=startKey&end=endKey"
    ```

- **Success Response**:
    - **HTTP Status**: `200 OK`
    - **Response Body**: A JSON array of key-value pairs
    ```json
    [
      {"key": "startKey", "value": "value1"},
      {"key": "middleKey", "value": "value2"},
      {"key": "endKey", "value": "value3"}
    ]
    ```

- **Error Response**:
    - If there’s an issue with the range: `500 Internal Server Error`
    - If the `start` or `end` parameters are missing: `400 Bad Request`
    - If the HTTP method is incorrect: `405 Method Not Allowed`

---

### 4. `/batchput` Route - Insert Multiple Key-Value Pairs

- **HTTP Method**: `POST`
- **Endpoint**: `/batchput`
- **Expected Payload**: A JSON object where each key maps to a value.
- **Sample Request**:
    ```bash
    curl -X POST "http://localhost:8080/batchput" -H "Content-Type: application/json" -d '{"key1": "value1", "key2": "value2", "key3": "value3"}'
    ```

- **Success Response**:
    - **HTTP Status**: `200 OK`

- **Error Response**:
    - If there’s an issue decoding the JSON: `400 Bad Request`
    - If there’s an error storing one or more key-value pairs: `500 Internal Server Error`
    - If the HTTP method is incorrect: `405 Method Not Allowed`

---

### 5. `/delete` Route - Delete a Key

- **HTTP Method**: `DELETE`
- **Endpoint**: `/delete?key={key}`
- **Expected Payload**: The `key` is provided as a query parameter.
- **Sample Request**:
    ```bash
    curl -X DELETE "http://localhost:8080/delete?key=myKey"
    ```

- **Success Response**:
    - **HTTP Status**: `200 OK`

- **Error Response**:
    - If the key is not found: `500 Internal Server Error`
    - If the `key` parameter is missing: `400 Bad Request`
    - If the HTTP method is incorrect: `405 Method Not Allowed`

#### Areas of Improvements

- **Compression:** Log Files and Indexes Compression to reduce memory
- **Index eviction:** Evict frequently used keys to reduce memory usage
- **Garbage Collection**: Cleaning up of deleted keys. They are currently marked with tombstone markers
- **Log compaction**  Compact log to remove deleted entries
- **Segmented Indexing:** Divide index to chunks and load chunks as they are needed
- **Node Discovery:** Use a separate service or gossip protocol to discover nodes in the cluster. Current approach can introduce split brain in some edge cases
- **Dynamic Leader election:**. Start all nodes as followers and elect leader dynamically after node discovery
- **Authentication Middleware:** Access is currently open. We need to protect the service with authentication





