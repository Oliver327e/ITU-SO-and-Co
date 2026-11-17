package replication

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	auctionpb "github.com/oliver327e/handin_5/api"
	"github.com/oliver327e/handin_5/internal/auction"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type savedBid struct {
	BidderID string
	Amount   int64
	Outcome  auction.BidOutcome
}

type snapshot struct {
	Auction  auction.Auction
	Requests map[string]savedBid
	Version  uint64
}

type Replica struct {
	auctionpb.UnimplementedAuctionServiceServer
	auctionpb.UnimplementedReplicationServiceServer

	mutex       sync.Mutex
	isPrimary   bool
	state       snapshot
	peerAuction auctionpb.AuctionServiceClient
	peerReplica auctionpb.ReplicationServiceClient
}

func New(
	isPrimary bool,
	auctionState *auction.Auction,
	connection *grpc.ClientConn,
) *Replica {
	return &Replica{
		isPrimary: isPrimary,

		state: snapshot{
			Auction:  *auctionState,
			Requests: make(map[string]savedBid),
		},

		peerAuction: auctionpb.NewAuctionServiceClient(connection),
		peerReplica: auctionpb.NewReplicationServiceClient(connection),
	}
}

func (replica *Replica) Bid(
	ctx context.Context,
	request *auctionpb.BidRequest,
) (*auctionpb.BidResponse, error) {
	if !replica.isPrimary {
		return replica.peerAuction.Bid(ctx, request)
	}

	requestID := request.GetRequestId()
	bidderID := request.GetBidderId()
	amount := request.GetAmount()

	if requestID == "" || bidderID == "" {
		return nil, status.Error(
			codes.InvalidArgument,
			"request ID and bidder ID are required",
		)
	}

	replica.mutex.Lock()
	defer replica.mutex.Unlock()

	saved, found := replica.state.Requests[requestID]

	if found {
		if saved.BidderID != bidderID || saved.Amount != amount {
			return nil, status.Error(
				codes.InvalidArgument,
				"request ID was already used for a different bid",
			)
		}
	} else {
		outcome := replica.state.Auction.Bid(bidderID, amount)

		saved = savedBid{
			BidderID: bidderID,
			Amount:   amount,
			Outcome:  outcome,
		}

		replica.state.Requests[requestID] = saved
		replica.state.Version++
	}

	// Even a retry must wait until the backup has the state.
	err := replica.copyToBackup(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	response := &auctionpb.BidResponse{}

	switch saved.Outcome {
	case auction.BidSuccess:
		response.Outcome = auctionpb.BidOutcome_BID_OUTCOME_SUCCESS
	case auction.BidFail:
		response.Outcome = auctionpb.BidOutcome_BID_OUTCOME_FAIL
	default:
		response.Outcome = auctionpb.BidOutcome_BID_OUTCOME_EXCEPTION
	}

	return response, nil
}

func (replica *Replica) Result(
	ctx context.Context,
	request *auctionpb.ResultRequest,
) (*auctionpb.ResultResponse, error) {
	if !replica.isPrimary {
		return replica.peerAuction.Result(ctx, request)
	}

	replica.mutex.Lock()
	defer replica.mutex.Unlock()

	err := replica.copyToBackup(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	result := replica.state.Auction.Result()

	return &auctionpb.ResultResponse{
		Finished:      result.Finished,
		HighestBid:    result.HighestBid,
		HighestBidder: result.HighestBidder,
	}, nil
}

func (replica *Replica) copyToBackup(ctx context.Context) error {
	data, err := json.Marshal(replica.state)
	if err != nil {
		return err
	}

	replicationContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	request := &auctionpb.StateUpdate{
		State: data,
	}

	_, err = replica.peerReplica.UpdateState(replicationContext, request)
	return err
}

func (replica *Replica) UpdateState(
	_ context.Context,
	request *auctionpb.StateUpdate,
) (*auctionpb.StateAck, error) {
	if replica.isPrimary {
		return nil, status.Error(
			codes.FailedPrecondition,
			"state updates must go to the backup",
		)
	}

	var incoming snapshot

	err := json.Unmarshal(request.GetState(), &incoming)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}

	replica.mutex.Lock()
	defer replica.mutex.Unlock()

	if incoming.Version >= replica.state.Version {
		replica.state = incoming
	}

	return &auctionpb.StateAck{}, nil
}
