package main

import (
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	chatpb "github.com/oliver327e/chit-chat/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type participant struct {
	outgoingMessages chan *chatpb.Broadcast
	disconnectSignal chan struct{}
}

type chatServer struct {
	chatpb.UnimplementedChatServiceServer
	mutex        sync.Mutex
	participants map[string]*participant
	logicalClock uint64
}

func (server *chatServer) Join(request *chatpb.JoinRequest, stream chatpb.ChatService_JoinServer) error {
	participantID := strings.TrimSpace(request.GetParticipantId())

	if participantID == "" {
		return status.Error(codes.InvalidArgument, "missing client id")
	}

	newParticipant := &participant{
		outgoingMessages: make(chan *chatpb.Broadcast, 32),
		disconnectSignal: make(chan struct{}),
	}

	server.mutex.Lock()

	if _, exists := server.participants[participantID]; exists {
		server.mutex.Unlock()
		return status.Error(codes.AlreadyExists, "client already joined")
	}

	// update lamport clock
	if request.GetLogicalTime() > server.logicalClock {
		server.logicalClock = request.GetLogicalTime()
	}
	server.logicalClock++

	server.participants[participantID] = newParticipant

	server.logicalClock++
	logicalTime := server.logicalClock

	joinMessage := &chatpb.Broadcast{
		Content:       fmt.Sprintf("Participant %s joined Chit Chat at logical time %d", participantID, logicalTime),
		LogicalTime:   logicalTime,
		ParticipantId: participantID,
		EventType:     chatpb.BroadcastType_BROADCAST_TYPE_JOIN,
	}

	// send join message to everyone
	for otherParticipantID, otherParticipant := range server.participants {
		select {
		case otherParticipant.outgoingMessages <- joinMessage:
		default:
			delete(server.participants, otherParticipantID)
			close(otherParticipant.disconnectSignal)
			log.Printf("Server - slow_client client_id=%s", otherParticipantID)
		}
	}

	server.mutex.Unlock()

	log.Printf("Server - client_connected client_id=%s", participantID)
	log.Printf("Server - broadcast_join client_id=%s logical_time=%d", participantID, logicalTime)

	defer func() {
		server.mutex.Lock()

		if server.participants[participantID] == newParticipant {
			delete(server.participants, participantID)
		}

		server.mutex.Unlock()

		log.Printf("Server - client_disconnected client_id=%s", participantID)
	}()

	for {
		select {
		case <-stream.Context().Done():
			return nil

		case <-newParticipant.disconnectSignal:
			return status.Error(codes.ResourceExhausted, "client too slow")

		case broadcastMessage := <-newParticipant.outgoingMessages:
			if err := stream.Send(broadcastMessage); err != nil {
				return err
			}

			log.Printf(
				"Server - message_sent client_id=%s logical_time=%d",
				participantID,
				broadcastMessage.GetLogicalTime(),
			)
		}
	}
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Server - startup_failed error=%v", err)
	}

	grpcServer := grpc.NewServer()

	chatpb.RegisterChatServiceServer(grpcServer, &chatServer{
		participants: make(map[string]*participant),
	})

	shutdownSignals := make(chan os.Signal, 1)
	signal.Notify(shutdownSignals, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(shutdownSignals)

	go func() {
		<-shutdownSignals
		log.Println("Server - shutdown")
		grpcServer.Stop()
	}()

	log.Println("Server - startup on port=50051")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("Server - server_error error=%v", err)
	}
}
