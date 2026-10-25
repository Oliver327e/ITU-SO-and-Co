package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	chatpb "github.com/oliver327e/chit-chat/grpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	participantID := flag.String("id", "X", "client id")
	flag.Parse()

	logFile, err := os.OpenFile(
		fmt.Sprintf("logs/client-%s.log", *participantID),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		log.Fatal(err)
	}
	defer logFile.Close()

	log.SetOutput(io.MultiWriter(os.Stderr, logFile))
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	connection, err := grpc.NewClient(
		"localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer connection.Close()

	chatClient := chatpb.NewChatServiceClient(connection)

	var clockMutex sync.Mutex
	var logicalClock uint64

	// join the chat
	logicalClock++

	joinContext, cancelJoin := context.WithCancel(context.Background())
	defer cancelJoin()

	stream, err := chatClient.Join(joinContext, &chatpb.JoinRequest{
		ParticipantId: *participantID,
		LogicalTime:   logicalClock,
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("component=Client event=join_requested client_id=%s", *participantID)

	joined := make(chan struct{}, 1)
	streamClosed := make(chan struct{})

	// receive messages
	go func() {
		defer close(streamClosed)

		for {
			broadcastMessage, err := stream.Recv()
			if err != nil {
				log.Printf("component=Client event=disconnected client_id=%s error=%v", *participantID, err)
				return
			}

			clockMutex.Lock()

			if broadcastMessage.GetLogicalTime() > logicalClock {
				logicalClock = broadcastMessage.GetLogicalTime()
			}
			logicalClock++

			currentClock := logicalClock
			clockMutex.Unlock()

			fmt.Printf("\n[L=%d] %s\n> ",
				broadcastMessage.GetLogicalTime(),
				broadcastMessage.GetContent(),
			)

			log.Printf(
				"component=Client event=message_received client_id=%s sender_id=%s logical_time=%d local_clock=%d content=%q",
				*participantID,
				broadcastMessage.GetParticipantId(),
				broadcastMessage.GetLogicalTime(),
				currentClock,
				broadcastMessage.GetContent(),
			)

			if broadcastMessage.GetEventType() == chatpb.BroadcastType_BROADCAST_TYPE_JOIN &&
				broadcastMessage.GetParticipantId() == *participantID {
				select {
				case joined <- struct{}{}:
				default:
				}
			}
		}
	}()

	select {
	case <-joined:
		fmt.Println("You joined Chit Chat!")
	case <-streamClosed:
		log.Fatal("could not join the chat")
	}

	fmt.Println("Type a message and press Enter.")

	inputScanner := bufio.NewScanner(os.Stdin)
	fmt.Print("> ")

	for inputScanner.Scan() {
		messageText := inputScanner.Text()

		if strings.TrimSpace(messageText) == "" {
			fmt.Print("> ")
			continue
		}

		if strings.TrimSpace(messageText) == "/leave" {
			clockMutex.Lock()
			logicalClock++
			requestTime := logicalClock
			clockMutex.Unlock()

			leaveContext, cancelLeave := context.WithTimeout(
				context.Background(), 5*time.Second,
			)

			leaveResponse, err := chatClient.Leave(leaveContext, &chatpb.LeaveRequest{
				ParticipantId: *participantID,
				LogicalTime:   requestTime,
			})
			cancelLeave()

			if err != nil {
				fmt.Printf("Could not leave: %v\n", err)
				fmt.Print("> ")
				continue
			}

			clockMutex.Lock()

			if leaveResponse.GetLogicalTime() > logicalClock {
				logicalClock = leaveResponse.GetLogicalTime()
			}
			logicalClock++

			clockMutex.Unlock()

			if !leaveResponse.GetSuccess() {
				fmt.Println("Could not leave the chat")
				fmt.Print("> ")
				continue
			}

			log.Printf("component=Client event=left client_id=%s",
				*participantID)

			cancelJoin()
			fmt.Println("You left Chit Chat!")
			return
		}

		clockMutex.Lock()
		logicalClock++
		requestTime := logicalClock
		clockMutex.Unlock()

		// send message to server
		sendContext, cancelSend := context.WithTimeout(context.Background(), 5*time.Second)

		response, err := chatClient.Publish(sendContext, &chatpb.PublishRequest{
			ParticipantId: *participantID,
			Text:          messageText,
			LogicalTime:   requestTime,
		})
		cancelSend()

		if err != nil {
			fmt.Printf("Could not send: %v\n", err)
		} else {
			clockMutex.Lock()

			if response.GetLogicalTime() > logicalClock {
				logicalClock = response.GetLogicalTime()
			}
			logicalClock++

			clockMutex.Unlock()

			if !response.GetAccepted() {
				fmt.Println("Message was not accepted")
			}
		}

		fmt.Print("> ")
	}

	if err := inputScanner.Err(); err != nil {
		log.Printf("component=Client event=input_error error=%v", err)
	}
}
