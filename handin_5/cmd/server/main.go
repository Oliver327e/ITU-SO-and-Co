package main

import (
	"flag"
	"log"
	"net"
	"time"

	auctionpb "github.com/oliver327e/handin_5/api"
	"github.com/oliver327e/handin_5/internal/auction"
	"github.com/oliver327e/handin_5/internal/replication"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	address := flag.String("address", "127.0.0.1:50051", "Server address")
	peer := flag.String("peer", "127.0.0.1:50052", "Other server address")
	role := flag.String("role", "primary", "primary or backup")
	duration := flag.Duration("duration", 100*time.Second, "Auction duration")
	flag.Parse()

	if *role != "primary" && *role != "backup" {
		log.Fatal("role must be primary or backup")
	}

	if *duration <= 0 {
		log.Fatal("auction duration must be positive")
	}

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal(err)
	}

	connection, err := grpc.NewClient(
		*peer,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	auctionState := &auction.Auction{
		RegisteredBidders: make(map[string]bool),
		Deadline:          time.Now().Add(*duration),
	}

	replica := replication.New(
		*role == "primary",
		auctionState,
		connection,
	)

	grpcServer := grpc.NewServer()
	auctionpb.RegisterAuctionServiceServer(grpcServer, replica)
	auctionpb.RegisterReplicationServiceServer(grpcServer, replica)

	log.Println("Server listening on:", *address)
	log.Println("Role:", *role)

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal(err)
	}
}
