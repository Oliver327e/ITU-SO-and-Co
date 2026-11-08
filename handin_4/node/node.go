package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	chatpb "github.com/oliver327e/chit-chat/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

	requestMutex     sync.Mutex
	mutex            sync.Mutex
	clock            uint64
	state            nodeState
	requestTimestamp uint64
	deferredReplies  []chan struct{}
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

func (node *Node) RequestAccess(
	ctx context.Context,
	request *chatpb.AccessRequest,
) (*chatpb.AccessReply, error) {
	_, knownPeer := node.clients[request.GetNodeId()]
	if !knownPeer || request.GetTimestamp() == 0 {
		return nil, status.Error(
			codes.InvalidArgument,
			"unknown peer or zero timestamp",
		)
	}

	node.mutex.Lock()

	node.receiveTimestamp(request.GetTimestamp())

	log.Printf(
		"node=%d RECEIVE_REQUEST from=%d request=%d clock=%d",
		node.id,
		request.GetNodeId(),
		request.GetTimestamp(),
		node.clock,
	)

	myRequestFirst := node.state == wanted && requestComesFirst(
		node.requestTimestamp,
		node.id,
		request.GetTimestamp(),
		request.GetNodeId(),
	)

	if node.state == held || myRequestFirst {
		// Wait until our own request has finished.
		permission := make(chan struct{})
		node.deferredReplies = append(node.deferredReplies, permission)

		log.Printf(
			"node=%d DEFER to=%d request=%d",
			node.id,
			request.GetNodeId(),
			request.GetTimestamp(),
		)

		node.mutex.Unlock()

		select {
		case <-permission:
			// Our request is finished, so we can reply.
		case <-ctx.Done():
			return nil, status.FromContextError(ctx.Err()).Err()
		}

		node.mutex.Lock()
	}

	node.clock++

	reply := &chatpb.AccessReply{
		Timestamp: node.clock,
	}

	log.Printf(
		"node=%d SEND_REPLY to=%d request=%d clock=%d",
		node.id,
		request.GetNodeId(),
		request.GetTimestamp(),
		node.clock,
	)

	node.mutex.Unlock()

	return reply, nil
}

func (node *Node) enterCriticalSection(
	ctx context.Context,
	duration time.Duration,
) error {
	// Only one local request can run at a time.
	node.requestMutex.Lock()
	defer node.requestMutex.Unlock()

	if err := ctx.Err(); err != nil {
		return err
	}

	node.mutex.Lock()

	node.clock++
	node.requestTimestamp = node.clock
	requestTimestamp := node.requestTimestamp
	node.state = wanted

	log.Printf(
		"node=%d REQUEST request=%d peers=%d",
		node.id,
		requestTimestamp,
		len(node.clients),
	)

	node.mutex.Unlock()

	// Release delayed replies when this attempt finishes.
	defer node.release()

	requestContext, cancelRequests := context.WithCancel(ctx)
	defer cancelRequests()

	results := make(chan error, len(node.clients))

	for peerID, client := range node.clients {
		go func(peerID int32, client chatpb.MutualExclusionClient) {
			log.Printf(
				"node=%d SEND_REQUEST to=%d request=%d",
				node.id,
				peerID,
				requestTimestamp,
			)

			reply, err := client.RequestAccess(
				requestContext,
				&chatpb.AccessRequest{
					NodeId:    node.id,
					Timestamp: requestTimestamp,
				},
				grpc.WaitForReady(true),
			)

			if err == nil {
				node.mutex.Lock()

				node.receiveTimestamp(reply.GetTimestamp())

				log.Printf(
					"node=%d RECEIVE_REPLY from=%d request=%d clock=%d",
					node.id,
					peerID,
					requestTimestamp,
					node.clock,
				)

				node.mutex.Unlock()
			}

			results <- err
		}(peerID, client)
	}

	// Step 7: Wait for every peer to reply.
	for range node.clients {
		select {
		case err := <-results:
			if err != nil {
				return fmt.Errorf("request failed: %w", err)
			}

		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Step 8: Enter the critical section.
	node.mutex.Lock()

	node.state = held

	log.Printf(
		"node=%d ENTER_CS request=%d",
		node.id,
		requestTimestamp,
	)

	node.mutex.Unlock()

	timer := time.NewTimer(duration)
	defer timer.Stop()

	select {
	case <-timer.C:
	case <-ctx.Done():
	}

	node.mutex.Lock()

	log.Printf(
		"node=%d EXIT_CS request=%d",
		node.id,
		requestTimestamp,
	)

	node.mutex.Unlock()

	return ctx.Err()
}
