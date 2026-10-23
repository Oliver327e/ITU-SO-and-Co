
package main

import (
    "log"
    "net"
    "os"
    "os/signal"
    "syscall"

    chatpb "github.com/oliver327e/chit-chat/grpc"
    "google.golang.org/grpc"
)

// Our implementation of the ChatService.
type chatServer struct {
    chatpb.UnimplementedChatServiceServer
}

func main() {
    // Include timestamps in the logs.
    log.SetFlags(log.LstdFlags | log.Lmicroseconds)

    // Listen for TCP connections on port 50051.
    listener, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatalf("component=Server event=startup_failed error=%v", err)
    }

    // Create the gRPC server.
    grpcServer := grpc.NewServer()

    // Register our ChatService.
    chatpb.RegisterChatServiceServer(grpcServer, &chatServer{})

    // Handle shutdown signals.
    shutdown := make(chan os.Signal, 1)
    signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
    defer signal.Stop(shutdown)

    go func() {
        <-shutdown
        log.Println("component=Server event=shutdown")
        grpcServer.GracefulStop()
    }()

    // Log server startup.
    log.Println("component=Server event=startup port=50051")

    // Start accepting gRPC connections.
    if err := grpcServer.Serve(listener); err != nil {
        log.Fatalf("component=Server event=server_error error=%v", err)
    }
}
