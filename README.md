# goredis
> A high-performance, thread-safe, in-memory key-value database built from scratch in Go.

![Go Version](https://img.shields.io/badge/Go-1.21%2B-00ADD8?style=flat&logo=go)
![License](https://img.shields.io/badge/license-MIT-blue.svg)
![Status](https://img.shields.io/badge/status-active-success.svg)

**goredis** is a custom implementation of a Redis-like database. It was built to understand the core engineering concepts of databases, networking, and concurrency without relying on external frameworks. 

## Features
- **In-Memory Storage**: Ultra-fast data access using Go's native maps.
- **Thread-Safe**: Concurrent read/writes protected by `sync.RWMutex` and `sync/atomic`.
- **TCP Server**: Built-in networking using TCP port `6379`. Connect via `telnet` or `netcat`.
- **TTL & Expiry**: Supports key expiration with a background Goroutine worker for garbage collection.
- **Persistence (AOF)**: Append-Only File system ensures data is not lost when the server restarts.
- **Professional Architecture**: Clean separation of the database engine and the network layer.

## Quick Start

### Prerequisites
- Go 1.21 or higher installed on your machine.

### Installation
    git clone https://github.com/your-username/goredis.git
    cd goredis
    go run .

### Connecting to the Server
Open a new terminal window and connect using `telnet` or `netcat (nc)`:

    telnet localhost 6379
    # OR
    nc localhost 6379

## Supported Commands

| Command | Usage | Description |
| :--- | :--- | :--- |
| `PING` | `PING` | Test server connection |
| `SET` | `SET <key> <value> [EX <seconds>]` | Store a key-value pair (optional expiry) |
| `GET` | `GET <key>` | Retrieve value for a key |
| `DEL` | `DEL <key>` | Delete a key |
| `EXISTS` | `EXISTS <key>` | Check if a key exists |
| `TTL` | `TTL <key>` | Get remaining time to live |
| `EXPIRE` | `EXPIRE <key> <seconds>` | Set timeout on an existing key |
| `PERSIST` | `PERSIST <key>` | Remove timeout from a key |
| `KEYS` | `KEYS` | List all active keys (formatted table) |
| `DBSIZE` | `DBSIZE` | Count total keys |
| `FLUSHALL` | `FLUSHALL` | Remove all keys |
| `INFO` | `INFO` | View server statistics |

## Architecture and Learning Outcomes
This project was built from scratch to master Go concurrency and networking:
- **Concurrency:** Used `sync.RWMutex` for safe concurrent map access and `atomic.Int64` for tracking server metrics.
- **Goroutines:** Used a background ticker goroutine to sweep expired keys.
- **Networking:** Built a custom TCP listener that spawns a new goroutine for every client connection.
- **File I/O:** Implemented AOF (Append-Only File) logging to persist data to the disk.

## Roadmap
- [x] Core Engine (In-Memory)
- [x] Advanced CLI and Formatting
- [x] Concurrency and Thread-Safety
- [x] TTL (Time-To-Live) and Expiry
- [x] TCP Server (Networking)
- [x] Persistence (AOF)
- [ ] Implement RESP Protocol (Redis Serialization Protocol)
- [ ] Custom CLI Client (instead of telnet)

## Author
**Sagar** 
- LinkedIn: [Your LinkedIn URL]
- GitHub: [@your-username](https://github.com/your-username)

## License
This project is licensed under the MIT License - see the LICENSE file for details.