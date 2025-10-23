package main

import (
	"bufio"
	proto "chit-chat/grpc"
	"context"
	"fmt"
	"log"
	"os"
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

	stream, err := client.ReceiveMessage(context.Background(), &proto.JoinRequest{User: username})
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
				safeIncrement(vectorClock, username)
				for user, theirClocks := range chatMessage.VectorClocks.Clocks {
					myClock := vectorClock.Clocks[user]
					vectorClock.Clocks[user] = max(theirClocks, myClock)
				}

			}

			log.Println(chatMessage.User, ":", chatMessage.Message, "|", chatMessage.VectorClocks)
		}
	}()

	fmt.Println("type your message: ")
	for {
		scanner.Scan()
		message := scanner.Text()
		if message == ".quit" {
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
