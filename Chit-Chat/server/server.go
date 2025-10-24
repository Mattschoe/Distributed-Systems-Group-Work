package main

import (
	"bufio"
	proto "chit-chat/grpc"
	"context"
	"log"
	"net"
	"os"
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

	s.addClient(user, stream)

	var joinedVC *proto.VectorClocks
	if req.VectorClocks != nil {
		joinedVC = req.VectorClocks
	} else {
		joinedVC = &proto.VectorClocks{Clocks: make(map[string]int64)}
	}
	s.broadcastJoinMessage(user, joinedVC)

	<-stream.Context().Done()

	s.removeClient(user)
	s.broadcastLeaveMessage(user, &proto.VectorClocks{Clocks: make(map[string]int64)})

	return nil
}

func (s *grpcServer) Leave(context context.Context, request *proto.LeaveRequest) (*proto.Empty, error) {
	user := request.User

	s.removeClient(user)
	vc := request.VectorClocks
	if vc == nil {
		vc = &proto.VectorClocks{Clocks: make(map[string]int64)}
	}

	s.broadcastLeaveMessage(user, vc)
	return &proto.Empty{}, nil
}

func (s *grpcServer) addClient(user string, stream proto.ChitChat_ReceiveMessageServer) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.clients[user] = stream
	log.Printf("SERVER (USER_CREATED): Created new client: %s", user)
	return nil
}

func (s *grpcServer) removeClient(user string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	delete(s.clients, user)
	log.Printf("SERVER (USER_REMOVED): Removed: %s", user)
	return nil
}

func (s *grpcServer) broadcastMessage(message *proto.ChatMessage) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	log.Printf("SERVER (BROADCASTING_MESSAGE): Broadcasting to users:")
	for user, stream := range s.clients {
		err := stream.Send(message)
		log.Printf("  - %s, with clockcount: %d", user, message.VectorClocks.Clocks[user])

		if err != nil {
			go func(user string) {
				s.removeClient(user)
			}(user)
		}
	}
	log.Printf("------------")
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

func (s *grpcServer) broadcastJoinMessage(user string, vc *proto.VectorClocks) {
	message := &proto.ChatMessage{
		User:         user,
		Message:      "-- joins the chat --",
		Type:         proto.ChatMessage_JOIN,
		VectorClocks: vc,
	}
	s.mutex.Lock()
	s.messageHistory = append(s.messageHistory, message)
	s.mutex.Unlock()
	s.broadcastMessage(message)
}

func (s *grpcServer) broadcastLeaveMessage(user string, vc *proto.VectorClocks) {
	message := &proto.ChatMessage{
		User:         user,
		Message:      "-- left the chat --",
		Type:         proto.ChatMessage_LEAVE,
		VectorClocks: vc,
	}
	s.mutex.Lock()
	s.messageHistory = append(s.messageHistory, message)
	s.mutex.Unlock()
	s.broadcastMessage(message)
}

func main() {
	server := &grpcServer{clients: map[string]proto.ChitChat_ReceiveMessageServer{}}

	server.start_server()
}

func (s *grpcServer) start_server() {
	grpcServer := grpc.NewServer()
	listener, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatalf("SERVER (ERROR): %v", err)
	}

	proto.RegisterChitChatServer(grpcServer, s)

	shutdown := make(chan struct{})
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		log.Println("SERVER (STARTUP): Server running. Type '.quit' to stop")
		for scanner.Scan() {
			if scanner.Text() == ".quit" {
				close(shutdown)
				return
			}
		}
	}()

	go func() {
		<-shutdown
		log.Println("SERVER (SHUTDOWN): Shutting down server")
		grpcServer.GracefulStop()
	}()

	err = grpcServer.Serve(listener)
	if err != nil {
		log.Fatalf("SERVER (ERROR): %v", err)
	}
}
