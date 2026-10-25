package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"unicode/utf8"

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
			log.Printf("component=Server event=slow_client client_id=%s", otherParticipantID)
		}
	}

	server.mutex.Unlock()

	log.Printf("component=Server event=client_connected client_id=%s", participantID)
	log.Printf("component=Server event=broadcast_join client_id=%s logical_time=%d", participantID, logicalTime)

	defer func() {
		server.removeParticipant(participantID, newParticipant, 0)

		log.Printf("component=Server event=client_disconnected client_id=%s",
			participantID)
	}()

	for {
		select {
		case <-stream.Context().Done():
			return nil

		case <-newParticipant.disconnectSignal:
			return nil

		case broadcastMessage := <-newParticipant.outgoingMessages:
			if err := stream.Send(broadcastMessage); err != nil {
				return err
			}

			log.Printf(
				"component=Server event=message_sent client_id=%s logical_time=%d",
				participantID,
				broadcastMessage.GetLogicalTime(),
			)
		}
	}
}

func (server *chatServer) Publish(
	requestContext context.Context,
	request *chatpb.PublishRequest,
) (*chatpb.PublishResponse, error) {

	participantID := strings.TrimSpace(request.GetParticipantId())
	messageText := request.GetText()

	if !validMessage(messageText) {
		return nil, status.Error(codes.InvalidArgument, "message must be 1-128 UTF-8 characters")
	}

	server.mutex.Lock()

	if _, exists := server.participants[participantID]; !exists {
		server.mutex.Unlock()
		return nil, status.Error(codes.FailedPrecondition, "client has not joined")
	}

	// update lamport clock
	if request.GetLogicalTime() > server.logicalClock {
		server.logicalClock = request.GetLogicalTime()
	}
	server.logicalClock++

	server.logicalClock++
	logicalTime := server.logicalClock

	broadcastMessage := &chatpb.Broadcast{
		Content:       messageText,
		LogicalTime:   logicalTime,
		ParticipantId: participantID,
		EventType:     chatpb.BroadcastType_BROADCAST_TYPE_CHAT,
	}

	// send to all active clients
	for otherParticipantID, otherParticipant := range server.participants {
		select {
		case otherParticipant.outgoingMessages <- broadcastMessage:
		default:
			delete(server.participants, otherParticipantID)
			close(otherParticipant.disconnectSignal)
			log.Printf("component=Server event=slow_client client_id=%s", otherParticipantID)
		}
	}

	// timestamp for the reply
	server.logicalClock++
	responseTime := server.logicalClock

	server.mutex.Unlock()

	log.Printf(
		"component=Server event=broadcast_chat client_id=%s logical_time=%d content=%q",
		participantID, logicalTime, messageText,
	)

	return &chatpb.PublishResponse{
		Accepted:    true,
		LogicalTime: responseTime,
	}, nil
}

func (server *chatServer) removeParticipant(
	participantID string,
	expectedParticipant *participant,
	receivedTime uint64,
) (uint64, bool) {

	server.mutex.Lock()
	defer server.mutex.Unlock()

	activeParticipant, exists := server.participants[participantID]

	if !exists || (expectedParticipant != nil && activeParticipant != expectedParticipant) {
		return 0, false
	}

	if receivedTime > server.logicalClock {
		server.logicalClock = receivedTime
	}
	server.logicalClock++

	delete(server.participants, participantID)
	close(activeParticipant.disconnectSignal)

	server.logicalClock++
	logicalTime := server.logicalClock

	leaveMessage := &chatpb.Broadcast{
		Content:       fmt.Sprintf("Participant %s left Chit Chat at logical time %d", participantID, logicalTime),
		LogicalTime:   logicalTime,
		ParticipantId: participantID,
		EventType:     chatpb.BroadcastType_BROADCAST_TYPE_LEAVE,
	}

	// tell the remaining clients
	for otherParticipantID, otherParticipant := range server.participants {
		select {
		case otherParticipant.outgoingMessages <- leaveMessage:
		default:
			delete(server.participants, otherParticipantID)
			close(otherParticipant.disconnectSignal)
			log.Printf("component=Server event=slow_client client_id=%s", otherParticipantID)
		}
	}

	log.Printf("component=Server event=broadcast_leave client_id=%s logical_time=%d",
		participantID, logicalTime)

	return logicalTime, true
}

func (server *chatServer) Leave(
	requestContext context.Context,
	request *chatpb.LeaveRequest,
) (*chatpb.LeaveResponse, error) {

	participantID := strings.TrimSpace(request.GetParticipantId())

	if participantID == "" {
		return nil, status.Error(codes.InvalidArgument, "missing client id")
	}

	logicalTime, removed := server.removeParticipant(
		participantID,
		nil,
		request.GetLogicalTime(),
	)

	if !removed {
		return nil, status.Error(codes.NotFound, "client not found")
	}

	server.mutex.Lock()
	server.logicalClock++
	responseTime := server.logicalClock
	server.mutex.Unlock()

	log.Printf("component=Server event=client_left client_id=%s logical_time=%d",
		participantID, logicalTime)

	return &chatpb.LeaveResponse{
		Success:     true,
		LogicalTime: responseTime,
	}, nil
}

func validMessage(messageText string) bool {
	return utf8.ValidString(messageText) &&
		utf8.RuneCountInString(messageText) <= 128 &&
		strings.TrimSpace(messageText) != ""
}

func main() {
	logFile, err := os.OpenFile(
		"logs/server.log",
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	listener, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("component=Server event=startup_failed error=%v", err)
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
		log.Println("component=Server event=shutdown")
		grpcServer.Stop()
	}()

	log.Println("component=Server event=startup on port=50051")

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("component=Server event=server_error error=%v", err)
	}
}
