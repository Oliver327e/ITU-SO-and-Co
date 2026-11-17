package main

import (
	"context"
	"flag"
	"log"
	"time"

	auctionpb "github.com/oliver327e/handin_5/api"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	address := flag.String("address", "127.0.0.1:50051", "Server address")
	action := flag.String("action", "result", "Action: bid or result")
	bidderID := flag.String("bidder", "", "Your bidder ID")
	amount := flag.Int64("amount", 0, "Bid amount")
	requestID := flag.String("request-id", "", "Unique ID for this bid")
	flag.Parse()

	connection, err := grpc.NewClient(
		*address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	client := auctionpb.NewAuctionServiceClient(connection)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch *action {
	case "bid":
		if *bidderID == "" {
			log.Fatal("provide a bidder ID using -bidder")
		}

		request := &auctionpb.BidRequest{
			BidderId:  *bidderID,
			Amount:    *amount,
			RequestId: *requestID,
		}

		response, err := client.Bid(ctx, request)
		if err != nil {
			log.Fatal("Bid exception: ", err)
		}

		log.Println("Bid outcome:", response.Outcome)

	case "result":
		request := &auctionpb.ResultRequest{}

		response, err := client.Result(ctx, request)
		if err != nil {
			log.Fatal("Result exception: ", err)
		}

		log.Println("Auction finished:", response.Finished)
		log.Println("Highest bid:", response.HighestBid)
		log.Println("Highest bidder:", response.HighestBidder)

	default:
		log.Fatal("action must be bid or result")
	}
}
