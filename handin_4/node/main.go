package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"strings"
	"time"

	chatpb "github.com/oliver327e/chit-chat/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	nodeID := flag.Int("id", 0, "node ID from nodes.json")
	flag.Parse()

	if *nodeID <= 0 || int64(*nodeID) > math.MaxInt32 {
		log.Fatal("Please provide a positive node ID using -id")
	}

	logPath := fmt.Sprintf("logs/node-%d.log", *nodeID)

	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatal(err)
	}
	logFile, err := os.OpenFile(
		logPath,
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		log.Fatal(err)
	}

	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	peers, localAddress, err := loadPeers("nodes.json", int32(*nodeID))
	if err != nil {
		log.Fatal(err)
	}

	listener, err := net.Listen("tcp", localAddress)
	if err != nil {
		log.Fatal(err)
	}

	node := &Node{
		id:      int32(*nodeID),
		clients: make(map[int32]chatpb.MutualExclusionClient),
	}

	for _, peer := range peers {
		if peer.ID == node.id {
			continue
		}

		connection, err := grpc.NewClient(
			peer.Address,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			log.Fatal(err)
		}
		defer connection.Close()

		node.clients[peer.ID] = chatpb.NewMutualExclusionClient(connection)

		log.Printf(
			"Node %d prepared a client for node %d at %s\n",
			node.id,
			peer.ID,
			peer.Address,
		)
	}

	grpcServer := grpc.NewServer()
	chatpb.RegisterMutualExclusionServer(grpcServer, node)

	log.Printf(
		"Node %d is listening on %s. Found %d nodes in the config.\n",
		node.id,
		localAddress,
		len(peers),
	)

	go func() {
		if err := grpcServer.Serve(listener); err != nil {
			log.Printf("gRPC server stopped: %v", err)
		}
	}()
	defer grpcServer.Stop()

	fmt.Println("Type enter to request the critical section.")
	fmt.Println("Press Ctrl+C to stop the node.")

	scanner := bufio.NewScanner(os.Stdin)

	for scanner.Scan() {
		command := strings.TrimSpace(scanner.Text())

		if command != "enter" {
			fmt.Println("Type enter to request access.")
			continue
		}

		err := node.enterCriticalSection(
			context.Background(),
			3*time.Second,
		)
		if err != nil {
			log.Printf("Could not enter critical section: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Could not read input: %v", err)
	}
}
