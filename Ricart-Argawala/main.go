package main

import (
	// "context"
	"bufio"
	"context"
	"fmt"
	"log"
	pb "ricart-argawala/proto"
	"strconv"

	"net"
	"os"

	"google.golang.org/grpc"
	// "google.golang.org/grpc"
)

var lamportClock int64

type peer struct {
	pb.UnimplementedPeer2PeerServer
}

func (p *peer) SendMessage(ctx context.Context, request *pb.MessageRequest) (*pb.MessageResponce, error) {
	if request.LamportClock > lamportClock {
		log.Printf("lamportClock replaced by request clock as %d > %d", request.GetLamportClock(), lamportClock)
		lamportClock = request.GetLamportClock()
	}
	lamportClock += 1
	log.Printf("recived request message %s and incrementing lamport clock %d", request.GetContent(), lamportClock)
	lamportClock += 1
	log.Printf("sending responce message and incrementing lamport clock %d", lamportClock)
	return &pb.MessageResponce{Content: "message recived", LamportClock: lamportClock}, nil
}

func startServer(listenAddr string) {
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("falied to listen on %s %v", listenAddr, err)
	}
	grpcServer := grpc.NewServer()

	pb.RegisterPeer2PeerServer(grpcServer, &peer{})

	log.Printf("Server listning on %s", listenAddr)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("falied serve grpc server %v", err)
	}

}

func connectAndSendMessage(peerAddresses []string, message string) {
	if len(peerAddresses) == 0 {
		log.Printf("list is empty %d", len(peerAddresses))
		return
	}

	for _, peerAddress := range peerAddresses {
		log.Println("connecting to" + peerAddress)
		conn, err := grpc.NewClient(peerAddress, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("falied to connect to  %s %v", peerAddress, err)
		}
		defer conn.Close()

		client := pb.NewPeer2PeerClient(conn)

		log.Printf("current clock: %d", lamportClock)
		lamportClock += 1
		log.Printf("sending message to %s and incrementing lamport clock %d", peerAddress, lamportClock)

		response, err := client.SendMessage(context.Background(), &pb.MessageRequest{Content: message, LamportClock: lamportClock})
		if err != nil {
			log.Fatalf("failed to send message: %v", err)
		}
		if response.GetLamportClock() > lamportClock {
			log.Printf("lamportClock replaced by Response clock as %d > %d", response.GetLamportClock(), lamportClock)
			lamportClock = response.GetLamportClock()
		}
		lamportClock += 1
		log.Printf("recived response %s and incrementing lamport clock %d", response.GetContent(), lamportClock)
	}

}

func main() {
	var peerAddresses []string
	var max int64
	max = 5000
	lamportClock = 0

	fileName := "PeerList.txt"

	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR|os.O_APPEND, os.ModePerm)
	if err != nil {
		log.Fatal(err)
	}

	scanner := bufio.NewScanner(file)
	writer := bufio.NewWriter(file)

	for scanner.Scan() {
		var scannedPort = scanner.Text()

		scannedPortInt, err := strconv.ParseInt(scannedPort, 10, 64)
		if err != nil {
			log.Fatal(err)
		}

		if scannedPortInt > max {
			max = scannedPortInt
		}

		fmt.Println(scannedPortInt)
		peerAddresses = append(peerAddresses, "localhost:"+scannedPort)
	}

	listenPort := strconv.FormatInt(max+1, 10)
	fmt.Println(listenPort)
	if _, err := writer.WriteString(listenPort + "\n"); err != nil {
		log.Fatal(err)
	}

	err = writer.Flush()
	if err != nil {
		log.Fatal(err)
	}

	listenAddress := "0.0.0.0:" + listenPort

	go startServer(listenAddress)

	connectAndSendMessage(peerAddresses, "Hello from port "+listenPort)

	select {}
}
