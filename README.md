# Chit Chat

Chit Chat is a distributed chat application made in Go using gRPC and Protocol Buffers. Clients can join, send messages, and leave. Lamport clocks are used to keep track of logical time.

## How to run

Make sure Go is installed.
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

## Logs

The server and clients save their logs in the logs/ folder. The logs show connections, messages, disconnections, and logical timestamps.
