# Distributed Mutual Exclusion (Mandatory activity 4)

This project implements distributed mutual exclusion in Go using gRPC and Protocol Buffers. The nodes use the Ricart-Agrawala algorithm to coordinate access to a critical section. Lamport clocks and node IDs are used to order requests.

## How to run

Make sure Go 1.27.3 or newer is installed.

Open a terminal in the `handin_4` folder and install the dependencies if needed:

```sh
go mod tidy
```

Open three terminals in the same folder and start one node in each:

```sh
go run ./node -id 1
```

```sh
go run ./node -id 2
```

```sh
go run ./node -id 3
```

Each node is both a gRPC server and a client of the other nodes. The node IDs and addresses are stored in `nodes.json`. All nodes must use the same configuration.

## How to use

- Start all three nodes using the commands above.
- Each node listens on its configured address and prepares clients for the other nodes.
- Press **Ctrl+C** in each terminal to stop the nodes.

The request and reply logic is in `node/node.go`. Terminal input for starting a critical section request still needs to be connected in `node/main.go`.

## Testing

Run the tests:

```sh
go test ./...
```

The current node test checks that an earlier timestamp gets priority and that the lower node ID wins when timestamps are equal.

## Logs

The nodes print their startup and request-related messages in the terminal. The request logic logs events such as requests, replies, and critical section entry and exit.

The current `main.go` does not yet save node logs to files.



# Chit Chat (Mandatory activity 3)

Chit Chat is a distributed chat application made in Go using gRPC and Protocol Buffers. Clients can join, send messages, and leave. Lamport clocks are used to keep track of logical time.

## How to run

Make sure Go is installed. \
Install the dependencies if needed:

`go mod tidy`

Start the server:

`go run ./server`

Open three new terminals and start the clients:

`go run ./client -id X` \
`go run ./client -id Y` \
`go run ./client -id Z`

## How to use

- Type a message and press Enter to send it.
- Type `/leave` or enter `CTRL+C` to leave the chat.
- All active clients receive messages and join/leave notifications.
- Each message has a Lamport timestamp.

## Testing

Run the tests:

`go test ./...`

The test only test whether the server catches invalid messages.
A valid message is a UTF-8 encoded string with a maximum length of 128 characters.

## Logs

The server and clients save their logs in the logs/ folder. The logs show connections, messages, disconnections, and logical timestamps.
