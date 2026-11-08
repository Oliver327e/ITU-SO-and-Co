package main

import (
	"flag"
	"fmt"
	"log"
	"math"
	"net"

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

		fmt.Printf(
			"Node %d prepared a client for node %d at %s\n",
			node.id,
			peer.ID,
			peer.Address,
		)
	}

	grpcServer := grpc.NewServer()
	chatpb.RegisterMutualExclusionServer(grpcServer, node)

	fmt.Printf(
		"Node %d is listening on %s. Found %d nodes in the config.\n",
		node.id,
		localAddress,
		len(peers),
	)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatal(err)
	}
}
