package replication

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	auctionpb "github.com/oliver327e/handin_5/grpc"
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
	Closed   bool
}

type Replica struct {
	auctionpb.UnimplementedAuctionServiceServer
	auctionpb.UnimplementedReplicationServiceServer

	mutex          sync.Mutex
	isPrimary      bool
	ready          bool
	peerFailed     bool
	failureTimeout time.Duration
	state          snapshot
	peerAuction    auctionpb.AuctionServiceClient
	peerReplica    auctionpb.ReplicationServiceClient
}

func New(
	isPrimary bool,
	auctionState *auction.Auction,
	connection *grpc.ClientConn,
	failureTimeout time.Duration,
) *Replica {
	return &Replica{
		isPrimary:      isPrimary,
		failureTimeout: failureTimeout,
		state: snapshot{
			Auction:  *auctionState,
			Requests: make(map[string]savedBid),
		},

		peerAuction: auctionpb.NewAuctionServiceClient(connection),
		peerReplica: auctionpb.NewReplicationServiceClient(connection),
	}
}

func (replica *Replica) Run(ctx context.Context) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		replica.mutex.Lock()

		if !replica.ready {
			if replica.isPrimary {
				err := replica.copyToBackup(ctx)
				if err == nil {
					replica.ready = true
					log.Println("Primary ready: initial state copied to backup")
				}
			}
			replica.mutex.Unlock()
			continue
		}

		peerFailed := replica.peerFailed
		replica.mutex.Unlock()

		if peerFailed {
			continue
		}

		checkContext, cancel := context.WithTimeout(ctx, replica.failureTimeout)
		_, err := replica.peerReplica.Ping(
			checkContext,
			&auctionpb.PingRequest{},
			grpc.WaitForReady(true),
		)
		timedOut := checkContext.Err() == context.DeadlineExceeded
		cancel()

		if err != nil && timedOut && ctx.Err() == nil {
			replica.mutex.Lock()
			replica.peerFailed = true
			wasPrimary := replica.isPrimary
			replica.isPrimary = true
			replica.mutex.Unlock()

			if wasPrimary {
				log.Println("Backup failed: continuing without a backup")
			} else {
				log.Println("Primary failed: this backup is now the primary")
			}
		}
	}
}

func (replica *Replica) Ping(
	_ context.Context,
	_ *auctionpb.PingRequest,
) (*auctionpb.PingResponse, error) {
	// A busy auction must still be able to answer heartbeats.
	return &auctionpb.PingResponse{}, nil
}

func (replica *Replica) Bid(
	ctx context.Context,
	request *auctionpb.BidRequest,
) (*auctionpb.BidResponse, error) {
	replica.mutex.Lock()

	if !replica.ready {
		replica.mutex.Unlock()
		return nil, status.Error(codes.Unavailable, "replicas are still starting")
	}

	if !replica.isPrimary {
		replica.mutex.Unlock()
		return replica.peerAuction.Bid(ctx, request)
	}
	defer replica.mutex.Unlock()

	requestID := request.GetRequestId()
	bidderID := request.GetBidderId()
	amount := request.GetAmount()

	if requestID == "" || bidderID == "" {
		return nil, status.Error(codes.InvalidArgument, "request ID and bidder ID are required")
	}

	saved, found := replica.state.Requests[requestID]
	if found {
		if saved.BidderID != bidderID || saved.Amount != amount {
			return nil, status.Error(codes.InvalidArgument, "request ID was already used for a different bid")
		}
	} else {
		replica.closeIfExpired()
		outcome := auction.BidFail

		if replica.state.Closed {
			replica.state.Auction.RegisteredBidders[bidderID] = true
		} else {
			outcome = replica.state.Auction.Bid(bidderID, amount)
			replica.closeIfExpired()
		}

		saved = savedBid{BidderID: bidderID, Amount: amount, Outcome: outcome}
		replica.state.Requests[requestID] = saved
		replica.state.Version++
	}

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
	replica.mutex.Lock()

	if !replica.ready {
		replica.mutex.Unlock()
		return nil, status.Error(codes.Unavailable, "replicas are still starting")
	}

	if !replica.isPrimary {
		replica.mutex.Unlock()
		return replica.peerAuction.Result(ctx, request)
	}
	defer replica.mutex.Unlock()

	replica.closeIfExpired()
	err := replica.copyToBackup(ctx)
	if err != nil {
		return nil, status.Error(codes.Unavailable, err.Error())
	}

	return &auctionpb.ResultResponse{
		Finished:      replica.state.Closed,
		HighestBid:    replica.state.Auction.HighestBid,
		HighestBidder: replica.state.Auction.HighestBidder,
	}, nil
}

func (replica *Replica) closeIfExpired() {
	if !replica.state.Closed && !time.Now().Before(replica.state.Auction.Deadline) {
		replica.state.Closed = true
		replica.state.Version++
	}
}

func (replica *Replica) copyToBackup(ctx context.Context) error {
	if replica.peerFailed {
		return nil
	}

	data, err := json.Marshal(replica.state)
	if err != nil {
		return err
	}

	replicationContext, cancel := context.WithTimeout(ctx, replica.failureTimeout)
	defer cancel()

	_, err = replica.peerReplica.UpdateState(
		replicationContext,
		&auctionpb.StateUpdate{State: data},
	)
	return err
}

func (replica *Replica) UpdateState(
	_ context.Context,
	request *auctionpb.StateUpdate,
) (*auctionpb.StateAck, error) {
	var incoming snapshot
	err := json.Unmarshal(request.GetState(), &incoming)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid state")
	}

	replica.mutex.Lock()
	defer replica.mutex.Unlock()

	if replica.isPrimary {
		return nil, status.Error(codes.FailedPrecondition, "this server is already a primary")
	}

	if incoming.Version >= replica.state.Version {
		replica.state = incoming
	}

	if !replica.ready {
		replica.ready = true
		log.Println("Backup ready: initial state received")
	}

	return &auctionpb.StateAck{}, nil
}
