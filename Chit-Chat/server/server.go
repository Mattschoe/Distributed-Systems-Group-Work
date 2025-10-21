package main

import (
	proto "chit-chat/grpc"
	"context"
	"log"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
)

type grpcServer struct {
	proto.UnimplementedChitChatServer
	clients        map[string]proto.ChitChat_ReceiveMessageServer
	mutex          sync.RWMutex
	messageHistory []*proto.ChatMessage
}

func (s *grpcServer) GetTime(ctx context.Context, in *proto.Empty) (*proto.TimeOld, error) {
	time2 := time.Now().UnixNano()
	time.Sleep(100 * time.Millisecond)
	time3 := time.Now().UnixNano()
	return &proto.TimeOld{Time2: time2, Time3: time3}, nil
}

func (s *grpcServer) ReceiveMessage(req *proto.JoinRequest, stream proto.ChitChat_ReceiveMessageServer) error {
	user := req.User

	s.broadcastJoinMessage(user)
	s.addClient(user, stream)

	<-stream.Context().Done()

	s.removeClient(user)
	s.broadcastLeaveMessage(user)

	return nil
}

func (s *grpcServer) addClient(user string, stream proto.ChitChat_ReceiveMessageServer) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.clients[user] = stream
	log.Printf("created new client %s", user)
	return nil
}

func (s *grpcServer) removeClient(user string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.clients, user)
	log.Printf("deleted %s", user)
	return nil
}

func (s *grpcServer) broadcastMessage(message *proto.ChatMessage) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if message.VectorClocks != nil && message.VectorClocks.Clocks != nil {
		for user := range message.VectorClocks.Clocks {
			log.Printf("-----------")
			log.Printf("user %s -> %d clockCount", user, message.VectorClocks.Clocks[user])
			log.Printf("-----------")
		}
	}

	for user, stream := range s.clients {
		if err := stream.Send(message); err != nil {
			go func(user string) {
				s.removeClient(user)
			}(user)
		}
	}
}

func (s *grpcServer) SendMessage(ctx context.Context, in *proto.SendMessageRequest) (*proto.Time, error) {
	var message = &proto.ChatMessage{User: in.User,
		Message:      in.Message,
		Type:         proto.ChatMessage_REGULAR,
		VectorClocks: in.VectorClocks,
	}

	s.messageHistory = append(s.messageHistory, message)

	s.broadcastMessage(message)

	return &proto.Time{Time: time.Now().UnixNano()}, nil
}

func (s *grpcServer) broadcastJoinMessage(user string) {
	message := &proto.ChatMessage{User: user,
		Message: "-- joins the chat --",
		Type:    proto.ChatMessage_JOIN,
		VectorClocks: &proto.VectorClocks{
			Clocks: make(map[string]int64),
		},
	}

	s.messageHistory = append(s.messageHistory, message)

	s.broadcastMessage(message)
}

func (s *grpcServer) broadcastLeaveMessage(user string) {

	s.messageHistory = append(s.messageHistory, &proto.ChatMessage{User: user,
		Message: "-- left the chat --",
		Type:    proto.ChatMessage_LEAVE,
	})
}

func main() {
	server := &grpcServer{clients: map[string]proto.ChitChat_ReceiveMessageServer{}}

	server.start_server()
}

func (s *grpcServer) start_server() {
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatalf("no")
	}

	proto.RegisterChitChatServer(grpcServer, s)
	log.Print("Server ready to accept connections")

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatal("no")
	}

}
