package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	pb "ricart-argawala/proto"
	"strconv"

	"net"
	"os"
	"time"

	"google.golang.org/grpc"
)

var lamportClock int64
var peerAddresses []string
var myRequestClock int64
var myAddress string
var repliesReceived int
var deferredChannels map[string]chan bool

type peer struct {
	pb.UnimplementedPeer2PeerServer
}

func (p *peer) SendMessage(ctx context.Context, request *pb.MessageRequest) (*pb.MessageResponce, error) {
	peerAddressesMerge(request.GetKnownAddresses())
	log.Printf("My address: %s, PeerAddresses list: %v", myAddress, peerAddresses)
	if request.LamportClock > lamportClock {
		log.Printf("lamportClock replaced by request clock as %d > %d", request.GetLamportClock(), lamportClock)
		lamportClock = request.GetLamportClock()
	}
	lamportClock += 1
	log.Printf("recived request message %s and incrementing lamport clock %d", request.GetContent(), lamportClock)
	lamportClock += 1
	log.Printf("sending responce message and incrementing lamport clock %d", lamportClock)
	peersToSend := append(peerAddresses, myAddress)
	return &pb.MessageResponce{Content: "message recived", LamportClock: lamportClock, KnownAddresses: peersToSend}, nil
}

func (p *peer) RequestCriticalSection(ctx context.Context, request *pb.CriticalSectionRequest) (*pb.CriticalSectionResponce, error) {
	if request.LamportClock > lamportClock {
		log.Printf("lamportClock replaced by request clock as %d > %d", request.GetLamportClock(), lamportClock)
		lamportClock = request.GetLamportClock()
	}
	lamportClock += 1
	log.Printf("received critical section request from %s, lamport clock: %d", request.GetRequesterAddress(), lamportClock)
	log.Printf("my request clock %d, request lamport clock: %d", myRequestClock, request.LamportClock)

	shouldReplyNow := myRequestClock == 0 || request.LamportClock < myRequestClock

	if shouldReplyNow {
		lamportClock += 1
		log.Printf("Replying immediately to %s, lamport clock: %d", request.GetRequesterAddress(), lamportClock)
		return &pb.CriticalSectionResponce{LamportClock: lamportClock}, nil
	} else {
		log.Printf("Deferring reply to %s (our request: %d, their request: %d)",
			request.GetRequesterAddress(), myRequestClock, request.LamportClock)

		ch := make(chan bool)
		deferredChannels[request.GetRequesterAddress()] = ch

		<-ch

		lamportClock += 1
		log.Printf("Sending deferred reply to %s, lamport clock: %d", request.GetRequesterAddress(), lamportClock)
		return &pb.CriticalSectionResponce{LamportClock: lamportClock}, nil
	}
}

func requestCriticalSection() {
	if len(peerAddresses) == 0 {
		log.Printf("No peers, entering CRITICAL SECTION immediately")
		log.Printf("*** ENTERED CRITICAL SECTION ***")
		return
	}

	lamportClock += 1
	myRequestClock = lamportClock
	repliesReceived = 0
	log.Printf("Requesting critical section from %d peers with clock %d", len(peerAddresses), myRequestClock)

	for _, peerAddress := range peerAddresses {
		log.Println("connecting to " + peerAddress)
		conn, err := grpc.NewClient(peerAddress, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("failed to connect to %s: %v", peerAddress, err)
		}

		client := pb.NewPeer2PeerClient(conn)

		log.Printf("sending CRITICAL SECTION request to %s, lamport clock: %d", peerAddress, myRequestClock)

		response, err := client.RequestCriticalSection(context.Background(), &pb.CriticalSectionRequest{
			LamportClock:     myRequestClock,
			RequesterAddress: myAddress})

		if err != nil {
			log.Fatalf("failed to send CRITICAL SECTION request: %v", err)
		}

		conn.Close()

		if response.GetLamportClock() > lamportClock {
			lamportClock = response.GetLamportClock()
		}
		lamportClock += 1
		repliesReceived++
		log.Printf("received reply %d/%d, lamport clock: %d", repliesReceived, len(peerAddresses), lamportClock)
	}

	log.Printf("*** ENTERED CRITICAL SECTION *** (received all %d replies)", repliesReceived)

	time.Sleep(10 * time.Second)

	log.Printf("*** EXITING CRITICAL SECTION ***")

	if len(deferredChannels) > 0 {
		log.Printf("Releasing %d deferred replies", len(deferredChannels))
		for addr, ch := range deferredChannels {
			log.Printf("Signaling deferred reply to %s", addr)
			close(ch)
		}
		deferredChannels = make(map[string]chan bool)
	}

	myRequestClock = 0
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

func connectAndSendMessage(message string) {
	if len(peerAddresses) == 0 {
		log.Printf("list is empty %d", len(peerAddresses))
		return
	}

	log.Printf("My address: %s, PeerAddresses list: %v", myAddress, peerAddresses)

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

		peersToSend := append(peerAddresses, myAddress)

		response, err := client.SendMessage(context.Background(), &pb.MessageRequest{Content: message,
			LamportClock:   lamportClock,
			KnownAddresses: peersToSend})
		if err != nil {
			log.Fatalf("failed to send message: %v", err)
		}
		peerAddressesMerge(response.GetKnownAddresses())
		if response.GetLamportClock() > lamportClock {
			log.Printf("lamportClock replaced by Response clock as %d > %d", response.GetLamportClock(), lamportClock)
			lamportClock = response.GetLamportClock()
		}
		lamportClock += 1
		log.Printf("recived response %s and incrementing lamport clock %d", response.GetContent(), lamportClock)
	}
}

func peerAddressesMerge(peerAddressesFromResponse []string) {
	uniquePeers := make(map[string]struct{})

	for _, addr := range peerAddresses {
		uniquePeers[addr] = struct{}{}
	}

	for _, addr := range peerAddressesFromResponse {
		uniquePeers[addr] = struct{}{}
	}

	result := make([]string, 0, len(uniquePeers))
	for addr := range uniquePeers {
		if addr != myAddress {
			result = append(result, addr)
		}
	}

	peerAddresses = result
}

func main() {
	var max int64
	max = 5000
	lamportClock = 0
	deferredChannels = make(map[string]chan bool)

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

	myAddress = "localhost:" + listenPort

	go startServer("0.0.0.0:" + listenPort)

	connectAndSendMessage("Hello from port " + listenPort)

	for len(peerAddresses) < 2 {
	}

	fmt.Println("you have 10 seconds to add more peers")

	time.Sleep(10 * time.Second)

	requestCriticalSection()

	select {}
}
