# Distributed Auction System

This project implements a distributed auction system in Go using gRPC and Protocol Buffers. The nodes use primary-backup replication. The primary handles bids and copies the auction state and saved responses to the backup before replying. The backup automatically takes over if the primary crashes.

## How to run

Make sure Go 1.27.3 or newer is installed.

Open a terminal in the `handin_5` folder and install the dependencies if needed:

```sh
go mod tidy
```

Open two terminals in the same folder and start one server in each.

Start the primary:

```sh
go run ./cmd/server -role=primary -address=127.0.0.1:50051 -peer=127.0.0.1:50052
```

Start the backup:

```sh
go run ./cmd/server -role=backup -address=127.0.0.1:50052 -peer=127.0.0.1:50051
```

The servers run as separate processes on the same computer. Each server knows the address of the other server. Clients can contact either server. While the primary is running, the backup forwards client requests to it.

## How to use

Start both servers using the commands above.

Wait until both servers print their ready messages.

Open a third terminal in the same folder and place a bid:

```sh
go run ./cmd/client -address=127.0.0.1:50052 -action=bid -bidder=alice -amount=10 -request-id=alice-1
```

Query the auction:

```sh
go run ./cmd/client -address=127.0.0.1:50052 -action=result
```

A new bid must be positive and higher than the current highest bid. Use a unique request ID for each new bid. When retrying the same bid, reuse its request ID to receive the saved response.

By default, the auction closes 100 seconds after the primary initializes. Before the deadline, the result shows the current highest bid and bidder. After the deadline, it shows the winner and winning amount. New bids are rejected after the deadline.

Press Ctrl+C in each server terminal to stop the servers.

## Testing

Run the Go test command:

```sh
go test ./...
```

To test automatic takeover manually, start both servers and place Alice's bid using the commands above. Add `-duration=5m` to the server commands if you need more time to test.

Press Ctrl+C in the primary's terminal. Wait until the backup prints:

```text
Primary failed: this backup is now the primary
```

Query the surviving server:

```sh
go run ./cmd/client -address=127.0.0.1:50052 -action=result
```

Alice's bid should still be present. Place a higher bid before the deadline:

```sh
go run ./cmd/client -address=127.0.0.1:50052 -action=bid -bidder=bob -amount=20 -request-id=bob-1
```

Query again to check that Bob is now the highest bidder. Repeating Alice's original bid command should return its saved success without changing Bob's higher bid.

To test backup failure, start a fresh pair of servers and stop the backup instead. Send requests to the surviving primary at `127.0.0.1:50051`.

The system supports one crash per run after both servers are ready. The default failure timeout is three seconds and must exceed the maximum healthy heartbeat response time, including processing. Requests during failure detection may return an exception and can be retried. Keep the crashed server stopped and start a fresh pair for another crash test.

## Logs

The servers print startup, synchronization, and failure-related messages in their terminals. The client prints bid outcomes and auction results.

Logs are printed to the terminal and are not saved to files.
