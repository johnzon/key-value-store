# Persistent Key/Value System Documentation

## Objective
To design and implement a network-available, persistent Key/Value store that adheres to the provided interface and achieves specific performance, reliability, and scalability requirements. The system is implemented in golang and uses only the standard library.

---

## Features

### Interfaces
1. **Put(Key, Value)**: Inserts or updates the value for the given key.
2. **Read(Key)**: Retrieves the value associated with the given key.
3. **ReadKeyRange(StartKey, EndKey)**: Fetches values for keys within the specified range.
4. **BatchPut(..keys, ..values)**: Allows multiple key-value pairs to be written in a single operation.
5. **Delete(Key)**: Removes the specified key and its associated value.

---

## Design Considerations

### Key Objectives
1. **Low Latency**: Optimized data structures and caching techniques for efficient read/write operations.
2. **High Throughput**: Batch processing and streamlined I/O handling for bulk operations.
3. **Scalability**: Support for datasets larger than RAM using persistent storage.
4. **Crash Resilience**: Write-ahead logging (WAL) and periodic snapshots to prevent data loss.
5. **Predictable Performance**: Rate limiting and connection pooling for stable performance under heavy loads.

### Bonus Features
1. **Replication**: Data is replicated across multiple nodes for fault tolerance.
2. **Failover Handling**: Automatic redirection of requests to healthy nodes during failures.

---

## Solution Components

### 1. **Storage Layer**

The storage module implements a **persistent key-value store** with the following features:

1. **Efficient Key Management**:
   - Keys and their file offsets are stored in a **sorted in-memory index** (`[]record`).
   - Binary search is used for quick lookups, ensuring efficient read and write operations.

2. **Persistence**:
   - Data is stored in an **append-only log file** for durability.
   - Each record is written in the format: `key|value\n`.

3. **Key-Range Queries**:
   - Supports reading key-value pairs within a specified range using the sorted index for fast range scans.

4. **Concurrency**:
   - Read-write locks (`sync.RWMutex`) protect the in-memory index, ensuring thread-safe operations.

5. **Crash Recovery**:
   - The append-only file guarantees durability, allowing recovery of data after crashes by reloading the log file and rebuilding the index.

6. **Optimized Write Behavior**:
   - New keys are inserted into the sorted index in logarithmic time, and existing keys are updated in place.

This design balances **performance**, **durability**, and **scalability**, making it suitable for production environments.

---

## Comparison of Key-Value Storage Approaches

### Two Approaches:
1. **Hash Map Approach** : A key-value store implemented using a hash map, where keys are mapped to file offsets.
2. **Sorted Slice Approach** : A key-value store implemented using a sorted slice of key-offset pairs, enabling efficient range queries and better scalability.

### Objective Comparison:

| Objective                                       | **Hash Map Approach**                          | **Sorted Slice Approach**                        |
|-------------------------------------------------|------------------------------------------------|-------------------------------------------------|
| **Low Latency per Read/Write**                  | O(1) for read/write                            | O(log n) for read, O(n) for write               |
| **High Throughput (Random Writes)**             | High throughput, constant time for writes      | Lower throughput due to sorting insertions      |
| **Large Datasets (Beyond RAM)**                 | Potential memory limitation, needs persistence | Handles larger-than-RAM with persistence, WAL   |
| **Crash Friendliness**                          | Needs WAL/snapshots for persistence            | Built-in crash recovery with WAL and snapshots  |
| **Range Queries**                               | Inefficient, requires sorting every query      | Efficient, can handle range queries efficiently |

### Analysis:

1. **Low Latency per Item**:
   - **Hash Map Approach**: Provides constant time complexity (O(1)) for both read and write operations, making it suitable for scenarios where low latency per operation is required.
   - **Sorted Slice Approach**: Requires binary search for reads (O(log n)) and sorting insertions (O(n)), which makes it less optimal for single operations but better suited for range queries.

2. **High Throughput (Writing Random Items)**:
   - **Hash Map Approach**: Since it uses constant time for writes, this approach is ideal for high-throughput, random writes, making it a good choice for environments with a high volume of data.
   - **Sorted Slice Approach**: Sorting the data on insertion incurs additional overhead, so it may not handle high-throughput random writes as efficiently as the hash map approach.

3. **Handling Large Datasets (Beyond RAM)**:
   - **Hash Map Approach**: Storing the entire dataset in memory limits scalability, making it harder to handle datasets larger than available RAM without additional persistence mechanisms.
   - **Sorted Slice Approach**: This approach can handle larger-than-RAM datasets by offloading data to disk and using techniques like write-ahead logs (WAL) and snapshots, making it more suitable for large-scale systems.

4. **Crash Friendliness**:
   - **Hash Map Approach**: To ensure crash recovery, the hash map implementation needs additional mechanisms such as WAL or snapshots to persist data.
   - **Sorted Slice Approach**: This approach inherently supports crash recovery through WAL and snapshot mechanisms, ensuring data integrity even during unexpected shutdowns.

5. **Range Queries**:
   - **Hash Map Approach**: Inefficient for range queries since the data is not sorted. Sorting the data for every range query would be a time-consuming process.
   - **Sorted Slice Approach**: Designed for efficient range queries. The sorted structure allows for quick binary search and retrieval of key ranges, making this approach ideal for handling range queries efficiently.

### Conclusion:
- **Hash Map Approach** is best suited for use cases where **low-latency individual reads and writes** are critical, and the data fits within memory limits.
- **Sorted Slice Approach** is more suitable for **range queries**, **large datasets**, and **crash recovery**, offering better scalability and persistence mechanisms.

For a system requiring efficient range queries, large-scale handling, and crash recovery, **the Sorted Slice Approach** is recommended. If the system is focused on high-speed, low-latency individual operations, **the Hash Map Approach** can be considered.

---

## Replication Layer Overview

The **Replication Layer** ensures data consistency and high availability across multiple nodes in the key-value storage system. It follows a **Leader-Follower** architecture:

1. **Leader Election**:
   - Electing a leader using Raft's algorithm.
   - Detecting leader failure and triggering failover to promote a new leader.
   
2. **Follower Replication**:
   - Follower nodes replicate the leader's data to maintain consistency.
   - Followers stay synchronized with the leader by applying the write-ahead logs (WAL) from the leader.

3. **Failover Handling**:
   - In case of leader failure, a new leader is automatically elected.
   - Followers detect leader failure and participate in the election process to ensure continued availability.

4. **Log Synchronization**:
   - Followers receive and apply the WAL from the leader to keep their data in sync.
   - This ensures all nodes have the same data for read operations.

---
## Raft Leader Election Implementation

### Components

#### 1. Leader Server
- **Handles Leader Election**: Manages the election process via `StartLeaderElection`.
- **Broadcasts Heartbeats**: Periodically sends heartbeats to followers.
- **Registers Followers**: Enables followers to connect for updates.

#### 2. Follower Server
- **Monitors Leader**: Detects leader failure through the `MonitorLeader` function.
- **Handles Heartbeats**: Updates leader status upon receiving heartbeats.
- **Initiates Failover**: Triggers a new election if the leader is inactive.

#### 3. Failover Manager
- **Triggers Leader Re-Election**: Handles failover with `TriggerFailover`.

---

### Workflow

1. **Leader Election**: A node initiates `StartLeaderElection` to become leader.
2. **Follower Monitoring**: Followers check heartbeats using `MonitorLeader`.
3. **Failover**: If the leader fails, the `FailoverManager` triggers a new election.

---

### Test Highlights

- Leader sends heartbeats.
- Follower detects leader failure and triggers a new election.
- System ensures seamless leader replacement.

---

### Configuration

- **Heartbeat Interval**: 2 seconds  
- **Heartbeat Timeout**: 10 seconds

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
│   │   ├── compaction.go   # Log compaction implementation (pending)
│   │   └── errors.go       # Custom storage-related errors (pending)
│   ├── transaction/        # Transaction layer
│   │   ├── wal.go          # Write-ahead logging
│   │   └── snapshot.go     # Snapshot management
│   ├── network/            # Networking layer
│   │   ├── server.go       # HTTP server setup
│   │   ├── handlers.go     # HTTP API handlers
│   │   └── middleware.go   # Middleware for logging, metrics, etc. (pending)
│   ├── replication/        # Replication layer (bonus)
│   │   ├── leader.go       # Leader logic
│   │   ├── follower.go     # Follower logic
│   │   └── failover.go     # Failover handling
├── pkg/
│   └── logger/             # Logging utilities
│       └── logger.go       # Logger setup and utilities (pending)
├── configs/
│   └── config.yaml         # Configuration file (e.g., ports, paths, etc.) (pending)
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

## Ongoing Improvements
1. Deployment: Adding docker to be able to run the application without installing go
2. More test coverage
3. Leader election : Ocassionally, previous failed leader instanceId is being returned as the leader Id. This is being resolved but keeping track of failed leaders. Submitted like this due to time constraint
4. Authentication Middleware

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

---

### Summary of Testing Steps

1. **Test `/put`**:
    - Send a PUT request with a `key` query parameter and the value as the request body.

2. **Test `/read`**:
    - Send a GET request with a `key` query parameter to retrieve the stored value.

3. **Test `/range`**:
    - Send a GET request with `start` and `end` key query parameters to retrieve a range of key-value pairs.

4. **Test `/batchput`**:
    - Send a POST request with a JSON body containing multiple key-value pairs.

5. **Test `/delete`**:
    - Send a DELETE request with a `key` query parameter to delete the specified key.

---



