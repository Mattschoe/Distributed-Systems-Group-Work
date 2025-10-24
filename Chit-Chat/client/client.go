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
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {

	var username string
	vectorClock := &proto.VectorClocks{
		Clocks: make(map[string]int64),
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

	safeIncrement(vectorClock, username)
	stream, err := client.ReceiveMessage(context.Background(), &proto.JoinRequest{User: username, VectorClocks: vectorClock})
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

			if chatMessage.VectorClocks != nil {
				for user, theirClocks := range chatMessage.VectorClocks.Clocks {
					if vectorClock.Clocks == nil {
						vectorClock.Clocks = make(map[string]int64)
					}
					if theirClocks > vectorClock.Clocks[user] {
						vectorClock.Clocks[user] = theirClocks
					}
				}
				vectorClock.Clocks[username] = vectorClock.Clocks[username] + 1
			}

			log.Println(chatMessage.User, ":", chatMessage.Message, "|", formatClock(chatMessage.VectorClocks))
		}
	}()

	fmt.Println("type your message: ")
	for {
		scanner.Scan()
		message := scanner.Text()
		if message == ".quit" {
			_, _ = client.Leave(context.Background(), &proto.LeaveRequest{User: username, VectorClocks: vectorClock})
			break
		}
		if len(message) > 180 {
			log.Println("Message too long, keep under 180 characters")
			break
		}
		safeIncrement(vectorClock, username)
		client.SendMessage(context.Background(),
			&proto.SendMessageRequest{User: username,
				Message:      message,
				VectorClocks: vectorClock})
		time.Sleep(2 * time.Second)
	}

	time.Sleep(2 * time.Second)
}
func safeIncrement(vc *proto.VectorClocks, user string) {
	if vc.Clocks == nil {
		vc.Clocks = make(map[string]int64)
	}
	vc.Clocks[user]++
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
