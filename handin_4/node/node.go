package main

import (
	"sync"

	chatpb "github.com/oliver327e/chit-chat/grpc"
)

type nodeState int

const (
	released nodeState = iota
	wanted
	held
)

type Node struct {
	chatpb.UnimplementedMutualExclusionServer

	id      int32
	clients map[int32]chatpb.MutualExclusionClient

	mutex            sync.Mutex
	clock            uint64
	state            nodeState
	requestTimestamp uint64
}

// Call this while holding the node's mutex.
func (node *Node) receiveTimestamp(timestamp uint64) {
	if timestamp > node.clock {
		node.clock = timestamp
	}

	node.clock++
}

func requestComesFirst(
	firstTimestamp uint64,
	firstID int32,
	secondTimestamp uint64,
	secondID int32,
) bool {
	if firstTimestamp != secondTimestamp {
		return firstTimestamp < secondTimestamp
	}

	return firstID < secondID
}
