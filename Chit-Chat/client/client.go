package main

import (
	"bufio"
	proto "chit-chat/grpc"
	"context"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SafeVectorClock struct {
	mutex        sync.Mutex
	vectorClocks *proto.VectorClocks
}

func main() {

	var username string
	clock := &SafeVectorClock{
		vectorClocks: &proto.VectorClocks{
			Clocks: make(map[string]int64),
		},
	}

	scanner := bufio.NewScanner(os.Stdin)
	conn, err := grpc.NewClient("localhost:8000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Connection failed: %v", err)
	}
	defer conn.Close()

	client := proto.NewChitChatClient(conn)

	fmt.Print("Type your username: ")
	fmt.Scan(&username)

	clock.Increment(username)
	stream, err := client.ReceiveMessage(context.Background(), &proto.JoinRequest{User: username, VectorClocks: clock.GetCopy()})
	if err != nil {
		log.Fatalf("Failed to join chat: %v", err)
	}

	go func() {
		for {
			chatMessage, err := stream.Recv()
			if err != nil {
				log.Println("Disconnected from chat:", err)
				return
			}

			//Only increments if it's not us (the same client) who has sent the message
			if chatMessage.User != username {
				clock.Merge(chatMessage.VectorClocks, username)
			}

			switch chatMessage.Type {
			case proto.ChatMessage_REGULAR:
				log.Println(chatMessage.User, ":", chatMessage.Message, "|", formatClock(chatMessage.VectorClocks))
			case proto.ChatMessage_JOIN:
				log.Println("[SERVER]", "User:", chatMessage.User, ",", "has joined the chat! :)")
			case proto.ChatMessage_LEAVE:
				log.Println("[SERVER]", "User:", chatMessage.User, ",", "Has left the chat! :(")
			}

		}
	}()

	fmt.Println("type your message: ")
	for {
		scanner.Scan()
		message := scanner.Text()
		if message == ".quit" {
			clock.Increment(username)
			_, _ = client.Leave(context.Background(), &proto.LeaveRequest{User: username, VectorClocks: clock.GetCopy()})
			break
		}
		if len(message) > 180 {
			log.Println("Message too long, keep under 180 characters")
			break
		}
		clock.Increment(username)
		client.SendMessage(
			context.Background(),
			&proto.SendMessageRequest{
				User:         username,
				Message:      message,
				VectorClocks: clock.GetCopy(),
			},
		)
	}
	time.Sleep(5 * time.Second)
	os.Exit(0)
}

func formatClock(vc *proto.VectorClocks) string {
	if vc == nil || len(vc.Clocks) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(vc.Clocks))
	for k, v := range vc.Clocks {
		parts = append(parts, fmt.Sprintf("%s:%d", k, v))
	}
	sort.Strings(parts)
	return "[" + strings.Join(parts, ", ") + "]"

}

func (s *SafeVectorClock) Increment(user string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.vectorClocks == nil {
		s.vectorClocks.Clocks = make(map[string]int64)
	}
	s.vectorClocks.Clocks[user]++
}

func (s *SafeVectorClock) Merge(incoming *proto.VectorClocks, self string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.vectorClocks == nil {
		return
	}
	for user, theirClock := range incoming.Clocks {
		if s.vectorClocks.Clocks == nil {
			s.vectorClocks.Clocks = make(map[string]int64)
		}
		if theirClock > s.vectorClocks.Clocks[user] {
			s.vectorClocks.Clocks[user] = theirClock
		}
	}
	s.vectorClocks.Clocks[self]++
}

// So we avoid deadlocks by only passing copies around
func (s *SafeVectorClock) GetCopy() *proto.VectorClocks {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	copy := &proto.VectorClocks{Clocks: make(map[string]int64)}
	for k, v := range s.vectorClocks.Clocks {
		copy.Clocks[k] = v
	}
	return copy
}
