package main

import (
	"context"
	"flag"
	"log"
	"net"
	"sync"
	"time"

	auctionpb "github.com/oliver327e/handin_5/api"
	"github.com/oliver327e/handin_5/internal/auction"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type auctionServer struct {
	auctionpb.UnimplementedAuctionServiceServer

	mutex        sync.Mutex
	auctionState *auction.Auction
}

func (server *auctionServer) Bid(
	_ context.Context,
	request *auctionpb.BidRequest,
) (*auctionpb.BidResponse, error) {
	if request.GetBidderId() == "" {
		return nil, status.Error(codes.InvalidArgument, "bidder ID is required")
	}

	server.mutex.Lock()
	defer server.mutex.Unlock()

	outcome := server.auctionState.Bid(
		request.GetBidderId(),
		request.GetAmount(),
	)

	response := &auctionpb.BidResponse{}

	switch outcome {
	case auction.BidSuccess:
		response.Outcome = auctionpb.BidOutcome_BID_OUTCOME_SUCCESS
	case auction.BidFail:
		response.Outcome = auctionpb.BidOutcome_BID_OUTCOME_FAIL
	case auction.BidException:
		response.Outcome = auctionpb.BidOutcome_BID_OUTCOME_EXCEPTION
	default:
		return nil, status.Error(codes.Internal, "unknown bid outcome")
	}

	return response, nil
}

func (server *auctionServer) Result(
	_ context.Context,
	_ *auctionpb.ResultRequest,
) (*auctionpb.ResultResponse, error) {
	server.mutex.Lock()
	defer server.mutex.Unlock()

	result := server.auctionState.Result()

	response := &auctionpb.ResultResponse{
		Finished:      result.Finished,
		HighestBid:    result.HighestBid,
		HighestBidder: result.HighestBidder,
	}

	return response, nil
}

func main() {
	address := flag.String("address", "127.0.0.1:50051", "Server address")
	duration := flag.Duration("duration", 100*time.Second, "Auction duration")
	flag.Parse()

	if *duration <= 0 {
		log.Fatal("auction duration must be positive")
	}

	listener, err := net.Listen("tcp", *address)
	if err != nil {
		log.Fatal(err)
	}

	auctionState := &auction.Auction{
		RegisteredBidders: make(map[string]bool),
		Deadline:          time.Now().Add(*duration),
	}

	server := &auctionServer{
		auctionState: auctionState,
	}

	grpcServer := grpc.NewServer()
	auctionpb.RegisterAuctionServiceServer(grpcServer, server)

	log.Println("Server listening on:", *address)
	log.Println("Auction closes at:", auctionState.Deadline)

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal(err)
	}
}
