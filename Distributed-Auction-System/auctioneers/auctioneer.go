package main

import (
	"context"
	pb "distributed-Auction-System/proto"
	"fmt"
	"log"
	"math/rand/v2"
	"strconv"

	"net"
	"time"

	"google.golang.org/grpc"
)

var lamportClock int64
var peerAddresses []string

// var myRequestClock int64
var myAddress string
var maxAddress int
var originalAddress string

// var repliesReceived int
var deferredChannels map[string]chan bool

var topBid int64
var winner string
var isAuctionRunning bool

type auctioneer struct {
	pb.UnimplementedAuctioneerServer
}

func (p *auctioneer) SendBid(ctx context.Context, request *pb.BidRequest) (*pb.BidResponce, error) {
	log.Printf("My address: %s, PeerAddresses list: %v", myAddress, peerAddresses)
	if request.LamportClock > lamportClock {
		log.Printf("lamportClock replaced by request clock as %d > %d", request.GetLamportClock(), lamportClock)
		lamportClock = request.GetLamportClock()
	}

	lamportClock += 1
	log.Printf("recived bid and incrementing lamport clock %d", lamportClock)

	if request.Bid > topBid {
		topBid = request.GetBid()
	} else {
		lamportClock += 1
		log.Printf("sending responce BidResponce_FAIL and incrementing lamport clock %d", lamportClock)

		return &pb.BidResponce{SuccesfulBid: pb.BidResponce_FAIL, LamportClock: lamportClock}, nil
	}

	lamportClock += 1
	log.Printf("sending responce BidResponce_SUCCESS and incrementing lamport clock %d", lamportClock)

	return &pb.BidResponce{SuccesfulBid: pb.BidResponce_SUCCESS, LamportClock: lamportClock}, nil
}

func (p *auctioneer) Results(ctx context.Context, request *pb.ResultRequest) (*pb.ResultResponce, error) {
	log.Printf("My address: %s, PeerAddresses list: %v", myAddress, peerAddresses)
	if request.LamportClock > lamportClock {
		log.Printf("lamportClock replaced by request clock as %d > %d", request.GetLamportClock(), lamportClock)
		lamportClock = request.GetLamportClock()
	}

	lamportClock += 1
	log.Printf("recived bid and incrementing lamport clock %d", lamportClock)

	lamportClock += 1
	log.Printf("sending responce BidResponce_SUCCESS and incrementing lamport clock %d", lamportClock)

	return &pb.ResultResponce{LamportClock: lamportClock, IsAuctionRunning: isAuctionRunning, Winner: winner, TopBid: topBid}, nil
}

func (p *auctioneer) JoinMesh(ctx context.Context, request *pb.JoinRequest) (*pb.JoinResponce, error) {
	peerAddressesMerge(request.GetKnownAddresses())
	log.Printf("My address: %s, PeerAddresses list: %v", myAddress, peerAddresses)
	if request.LamportClock > lamportClock {
		log.Printf("lamportClock replaced by request clock as %d > %d", request.GetLamportClock(), lamportClock)
		lamportClock = request.GetLamportClock()
	}
	lamportClock += 1
	log.Printf("recived request message and incrementing lamport clock %d", lamportClock)
	lamportClock += 1
	log.Printf("sending responce message and incrementing lamport clock %d", lamportClock)
	peersToSend := append(peerAddresses, myAddress)
	return &pb.JoinResponce{LamportClock: lamportClock, KnownAddresses: peersToSend}, nil
}

func (p *auctioneer) RemoveFromMesh(ctx context.Context, request *pb.RemoveRequest) (*pb.RemoveResponce, error) {
	log.Printf("My address: %s, pre remove PeerAddresses list: %v", myAddress, peerAddresses)
	if request.LamportClock > lamportClock {
		log.Printf("lamportClock replaced by request clock as %d > %d", request.GetLamportClock(), lamportClock)
		lamportClock = request.GetLamportClock()
	}
	lamportClock += 1
	log.Printf("recived request message and incrementing lamport clock %d", lamportClock)

	portToRemove := request.GetPortToRemove()
	for i, addr := range peerAddresses {
		if addr == portToRemove {
			peerAddresses = append(peerAddresses[:i], peerAddresses[i+1:]...)
			break
		}
	}

	peerAddresses = append(peerAddresses, request.GetPortToAdd())

	log.Printf("My address: %s, post remove PeerAddresses list: %v", myAddress, peerAddresses)

	lamportClock += 1
	log.Printf("sending responce message and incrementing lamport clock %d", lamportClock)
	return &pb.RemoveResponce{LamportClock: lamportClock}, nil
}

func startServer(originalAddress string) {

	myAddress = "localhost:" + originalAddress

	listenAddr := "0.0.0.0:" + originalAddress

	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		fmt.Println("original port already taken")

		min := 10000
		max := 50000

		peerAddresses = append(peerAddresses, "localhost:"+originalAddress)

		randPort := rand.IntN(max-min) + min

		randPortAsString := strconv.Itoa(randPort)

		myAddress = "localhost:" + randPortAsString

		listenAddr := "0.0.0.0:" + randPortAsString

		listener, err = net.Listen("tcp", listenAddr)
		if err != nil {
			log.Fatalf("falied to listen on %s %v", listenAddr, err)
		}

		grpcServer := grpc.NewServer()

		pb.RegisterAuctioneerServer(grpcServer, &auctioneer{})

		log.Printf("Server listning on %s", listenAddr)

		go connectToOtherAuctioneer(grpcServer, listener)

		if err := grpcServer.Serve(listener); err != nil {
			log.Printf("Server stopped: %v", err)
		}

		return
	}

	grpcServer := grpc.NewServer()

	pb.RegisterAuctioneerServer(grpcServer, &auctioneer{})

	log.Printf("Server listning on %s", listenAddr)

	if err := grpcServer.Serve(listener); err != nil {
		log.Fatalf("falied serve grpc server %v", err)
	}

	log.Println("there might be issuss when a new auctioneer replaces the original old one there might need to be some exploratio")
}

func connectToOtherAuctioneer(randomServer *grpc.Server, randomListener net.Listener) {
	if len(peerAddresses) == 0 {
		log.Printf("list is empty %d", len(peerAddresses))
		return
	}

	log.Printf("My address: %s, PeerAddresses list: %v", myAddress, peerAddresses)

	for i := 0; i < len(peerAddresses); i++ {
		log.Println("connecting to" + peerAddresses[i])
		conn, err := grpc.NewClient(peerAddresses[i], grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("falied to connect to  %s %v", peerAddresses[i], err)
		}
		defer conn.Close()

		client := pb.NewAuctioneerClient(conn)

		log.Printf("current clock: %d", lamportClock)
		lamportClock += 1
		log.Printf("sending message to %s and incrementing lamport clock %d", peerAddresses[i], lamportClock)

		peersToSend := append(peerAddresses, myAddress)

		response, err := client.JoinMesh(context.Background(), &pb.JoinRequest{
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
		log.Printf("recived response incrementing lamport clock %d", lamportClock)
	}

	log.Printf("My address random: %s, PeerAddresses list: %v", myAddress, peerAddresses)
	var randomMyAddress string = myAddress

	go startServer(strconv.Itoa(maxAddress + 1))

	time.Sleep(1 * time.Second)

	for _, peerAddress := range peerAddresses {
		log.Println("connecting to" + peerAddress)
		conn, err := grpc.NewClient(peerAddress, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			log.Fatalf("falied to connect to  %s %v", peerAddress, err)
		}
		defer conn.Close()

		client := pb.NewAuctioneerClient(conn)

		log.Printf("current clock: %d", lamportClock)
		lamportClock += 1
		log.Printf("sending message to %s and incrementing lamport clock %d", peerAddresses, lamportClock)

		response, err := client.RemoveFromMesh(context.Background(), &pb.RemoveRequest{
			LamportClock: lamportClock,
			PortToRemove: randomMyAddress,
			PortToAdd:    myAddress,
		})

		if err != nil {
			log.Fatalf("failed to send message: %v", err)
		}
		if response.GetLamportClock() > lamportClock {
			log.Printf("lamportClock replaced by Response clock as %d > %d", response.GetLamportClock(), lamportClock)
			lamportClock = response.GetLamportClock()
		}
		lamportClock += 1
		log.Printf("recived response incrementing lamport clock %d", lamportClock)
	}

	log.Printf("Stopping random server on %s", randomMyAddress)
	randomServer.GracefulStop()
	randomListener.Close()
	log.Printf("Random server stopped successfully")
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
			portStr := addr[len("localhost:"):]
			port, err := strconv.Atoi(portStr)
			if err == nil && port > maxAddress {
				maxAddress = port
			}
			result = append(result, addr)
		}
	}

	peerAddresses = result
}

func main() {
	lamportClock = 0

	topBid = 0
	winner = "no winner"
	isAuctionRunning = true

	originalAddress = "5000"
	deferredChannels = make(map[string]chan bool)

	go startServer(originalAddress)

	for len(peerAddresses) < 2 {
	}

	fmt.Println("you have 10 seconds to add more peers")

	time.Sleep(10 * time.Second)

	// requestCriticalSection()

	select {}
}
