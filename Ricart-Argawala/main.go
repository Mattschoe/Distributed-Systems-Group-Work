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
var deferredReplies []string
var myAddress string
var repliesReceived int

type peer struct {
	pb.UnimplementedPeer2PeerServer
}

func enqueue(queue []string, element string) []string {
	queue = append(queue, element)
	return queue
}

func dequeue(queue []string) (string, []string) {
	element := queue[0]
	if len(queue) == 1 {
		var tmp = []string{}
		return element, tmp
	}
	return element, queue[1:]
}

func peek(queue []string) string {
	if len(queue) == 0 {
		return ""
	}
	return queue[0]
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
		deferredReplies = append(deferredReplies, request.GetRequesterAddress())

		lamportClock += 1
		return &pb.CriticalSectionResponce{LamportClock: lamportClock}, nil
	}
}

func sendDeferredReplies() {
	for _, peerAddress := range deferredReplies {
		log.Printf("Sending deferred reply to %s", peerAddress)

		conn, err := grpc.NewClient(peerAddress, grpc.WithInsecure())
		if err != nil {
			log.Printf("failed to connect to %s: %v", peerAddress, err)
			continue
		}

		client := pb.NewPeer2PeerClient(conn)
		lamportClock += 1

		_, err = client.RequestCriticalSection(context.Background(), &pb.CriticalSectionRequest{
			LamportClock:     lamportClock,
			RequesterAddress: myAddress,
		})
		conn.Close()

		if err != nil {
			log.Printf("failed to send deferred reply: %v", err)
		}
	}
	deferredReplies = []string{}
}

func requestCriticalSection() {
	if len(peerAddresses) == 0 {
		log.Printf("No peers, entering CS immediately")
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

		log.Printf("sending CS request to %s, lamport clock: %d", peerAddress, myRequestClock)

		response, err := client.RequestCriticalSection(context.Background(), &pb.CriticalSectionRequest{
			LamportClock:     myRequestClock,
			RequesterAddress: myAddress})

		if err != nil {
			log.Fatalf("failed to send CS request: %v", err)
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

	if len(deferredReplies) > 0 {
		log.Printf("Sending %d deferred replies", len(deferredReplies))
		sendDeferredReplies()
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

	requestCriticalSection()

	select {}
}
