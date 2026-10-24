package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	chatpb "github.com/oliver327e/chit-chat/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	id := flag.String("id", "X", "client id")
	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	conn, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := chatpb.NewChatServiceClient(conn)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	clock := uint64(0)
	clock++

	stream, err := client.Join(ctx, &chatpb.JoinRequest{
		ParticipantId: *id,
		LogicalTime:   clock,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Client - join_requested client_id=%s", *id)

	for {
		msg, err := stream.Recv()
		if err != nil {
			log.Printf("Client - disconnected client_id=%s error=%v", *id, err)
			return
		}

		if msg.GetLogicalTime() > clock {
			clock = msg.GetLogicalTime()
		}
		clock++

		fmt.Printf("[L=%d] %s\n", msg.GetLogicalTime(), msg.GetContent())

		log.Printf(
			"Client - message_received client_id=%s logical_time=%d local_clock=%d content=%q",
			*id, msg.GetLogicalTime(), clock, msg.GetContent(),
		)
	}
}
