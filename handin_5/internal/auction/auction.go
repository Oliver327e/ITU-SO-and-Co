package auction

import "time"

type BidOutcome string

const (
	BidSuccess   BidOutcome = "success"
	BidFail      BidOutcome = "fail"
	BidException BidOutcome = "exception"
)

type Auction struct {
	RegisteredBidders map[string]bool
	HighestBid        int64
	HighestBidder     string
	Deadline          time.Time
}

func (auction *Auction) Bid(
	bidderID string,
	amount int64,
) BidOutcome {

	// Registers new Bidders
	if !auction.RegisteredBidders[bidderID] {
		auction.RegisteredBidders[bidderID] = true
	}
	// Deadline has passed
	if !time.Now().Before(auction.Deadline) {
		return BidFail
	}
	// Amount is insufficient
	if amount <= 0 || amount <= auction.HighestBid {
		return BidFail
	}

	auction.HighestBid = amount
	auction.HighestBidder = bidderID

	return BidSuccess
}

type AuctionResult struct {
	Finished      bool
	HighestBid    int64
	HighestBidder string
}

func (auction *Auction) Result() AuctionResult {
	return AuctionResult{
		Finished:      !time.Now().Before(auction.Deadline),
		HighestBid:    auction.HighestBid,
		HighestBidder: auction.HighestBidder,
	}
}
