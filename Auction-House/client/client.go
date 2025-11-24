package main

import (
	pb "Auction-House/grpc"
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rogpeppe/go-internal/lockedfile"
	"google.golang.org/grpc"
)

var filename string
var port string
var ID2Auction map[int32]*pb.Outcome
var auctionMutex sync.RWMutex

func main() {
	filename = filepath.Join(os.TempDir(), "LiveProcesses.txt")
	port = ":" + strconv.Itoa(rand.Intn(10_000))
	ID2Auction = make(map[int32]*pb.Outcome)
	listener, err := net.Listen("tcp", port)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	log.Printf("Client is online with ID: %s \n - Use '.Sell <product> <timeframe>' to start an auction. \n - Use '.Bid <auction_id> <amount>' to bid on a auction \n - Use '.Status' to see the status of auctions", port)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.Split(scanner.Text(), " ")

		if len(input) == 1 && (input[0] == ".Status" || input[0] == ".status") {
			requestStatus()
		} else if len(input) == 3 {
			if input[0] == ".Sell" || input[0] == ".sell" {
				product := input[1]
				timeframe, err := strconv.Atoi(input[2])
				if err != nil {
					log.Printf("Error parsing timeframe: %v \n", err)
					continue
				}
				go sendAuction(product, timeframe)
			} else if input[0] == ".Bid" || input[0] == ".bid" {
				auctionID, err := strconv.Atoi(input[1])
				if err != nil {
					fmt.Printf("Error parsing %s to auction id: %v \n", input[1], err)
					continue
				}
				amount, err := strconv.Atoi(input[2])
				if err != nil {
					fmt.Printf("Error parsing %s to amount: %v \n", input[2], err)
				}
				err = sendBid(auctionID, amount)
				if err != nil {
					fmt.Printf("Error sending bid: %v \n", err)
				}
			}
		} else {
			fmt.Println("Invalid input")
		}
	}
}

func requestStatus() {
	ports, err := readNetwork()
	if err != nil {
		fmt.Printf("Error reading network ports: %v \n", err)
		return
	}

	localAuctions := make(map[int32]*pb.Outcome)
	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v \n", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)
		auctions, err := client.Status(context.Background(), &pb.Empty{})
		if err != nil {
			fmt.Printf("Error getting status: %v \n", err)
		}
		for _, auction := range auctions.Auctions {
			localAuctions[auction.AuctionID] = auction
		}
	}

	fmt.Println("Current auctions exists:")
	for ID, auction := range localAuctions {
		fmt.Printf("%d: \n", ID)
		fmt.Printf("- status: %v \n", auction.Status.String())
		switch auction.Status {
		case pb.Outcome_Running:
			{
				fmt.Printf("- Currently winning: %d \n", auction.WinnerID)
				fmt.Printf("- Currently highest bid: %d \n", auction.CurrentBid)
			}
		case pb.Outcome_Finished:
			{
				fmt.Printf("- Winner: %d \n", auction.WinnerID)
				fmt.Printf("- Winning Bid: %d \n", auction.CurrentBid)
			}
		}
		fmt.Println("--------")
	}
}

// Holds a quorum agreeing to what the outcome of the auction should be
func holdQuorum(auctionID int32) {
	ports, err := readNetwork()
	if err != nil {
		fmt.Printf("Error reading network ports: %v \n", err)
		return
	}

	type outcomeKey struct {
		Status     pb.Outcome_AuctionStatus
		WinnerID   int32
		CurrentBid int32
		AuctionID  int32
	}

	outcome2Amount := make(map[outcomeKey]int)

	auctionMutex.RLock()
	auction := ID2Auction[auctionID]
	auctionMutex.RUnlock()

	//Gets outcome from every other port
	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v \n", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)
		clientOutcome, err := client.Result(context.Background(), auction)
		if err != nil {
			fmt.Printf("Error getting outcome from client!: %v \n", err)
			continue
		}
		//Skips clients who hasn't registered any bids
		if clientOutcome.WinnerID == -1 {
			continue
		}

		key := outcomeKey{
			Status:     pb.Outcome_Finished,
			WinnerID:   clientOutcome.WinnerID,
			CurrentBid: clientOutcome.CurrentBid,
			AuctionID:  clientOutcome.AuctionID,
		}

		outcome2Amount[key]++
	}

	//Finds most agreed outcome
	var highest = 0
	var winner outcomeKey
	for outcome, _ := range outcome2Amount {
		amount := outcome2Amount[outcome]
		if amount > highest {
			highest = amount
			winner = outcome
		}
	}
	newOutcome := &pb.Outcome{
		Status:     pb.Outcome_Finished,
		WinnerID:   winner.WinnerID,
		CurrentBid: winner.CurrentBid,
		AuctionID:  winner.AuctionID,
	}

	auctionMutex.Lock()
	ID2Auction[auctionID] = newOutcome
	auctionMutex.Unlock()

	fmt.Printf("Quorum is finished and the result is: %v \n", newOutcome)
	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v \n", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)
		ack, err := client.UpdateResult(context.Background(), newOutcome)
		if ack == nil || err != nil {
			fmt.Println("Failed with updating the auction result to the server!")
		}
	}
}

func sendBid(auctionID int, amount int) error {
	ports, err := readNetwork()
	if err != nil {
		fmt.Printf("Error reading network ports: %v", err)
		return err
	}

	processID, err := strconv.Atoi(strings.Split(port, ":")[1])
	if err != nil {
		return err
	}

	bidSuccess := true
	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to server at port %s: %v", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)

		ack, err := client.Bid(context.Background(), &pb.BidInfo{
			AuctionID: int32(auctionID),
			ProcessID: int32(processID),
			Amount:    int32(amount),
		})
		if err != nil {
			fmt.Printf("I couldn't communicate with server at port: %s :( \n", port)
			continue
		}

		switch ack.Status {
		case pb.BidAcknowledgement_Fail:
			{
				fmt.Println("Sending bid failed, try again!")
				bidSuccess = false
			}
		case pb.BidAcknowledgement_TooLow:
			{
				fmt.Println("You bid too low, try again!")
				bidSuccess = false
			}
		}
	}
	if bidSuccess {
		fmt.Println("Sending bid succeeded!")
		auctionMutex.Lock()
		ID2Auction[int32(auctionID)] = &pb.Outcome{
			Status:     pb.Outcome_Running,
			AuctionID:  int32(auctionID),
			WinnerID:   int32(processID),
			CurrentBid: int32(amount),
		}
		auctionMutex.Unlock()
	}
	return nil
}

// Returns true and auctionID if success and an auction has begun and false if a failure occured (like not enough acks)
func sendAuction(product string, timeframe int) {
	ports, err := readNetwork()
	if err != nil {
		fmt.Printf("Error reading network ports: %v \n", err)
		return
	}

	var acks []*pb.Outcome
	auctionID := int32(rand.Intn(10_000))

	auctionMutex.Lock()
	ID2Auction[auctionID] = &pb.Outcome{
		Status:     pb.Outcome_Running,
		WinnerID:   -1,
		CurrentBid: -1,
		AuctionID:  auctionID,
	}
	auctionMutex.Unlock()

	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v \n", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)

		go func() {
			ack, err := client.Sell(context.Background(), &pb.Auction{
				ID:        auctionID,
				Product:   product,
				Timeframe: int32(timeframe),
			})
			if err != nil {
				fmt.Printf("Error holding auction: %v \n", err)
				return
			}
			acks = append(acks, ack)
		}()
	}

	//Waits 1,5x auction time and then checks if the auction was a success
	fmt.Printf("Auction (with ID: %d) has started. You can now bid or wait \n", auctionID)
	time.Sleep(time.Duration(float32(timeframe)*1.5) * time.Second)
	var success int
	for _, ack := range acks {
		if ack.GetStatus() == pb.Outcome_Finished {
			success++
		}
	}
	if success >= len(ports)/2+1 {
		holdQuorum(auctionID)
	} else {
		fmt.Println("Auction has failed, not enough responses!")
	}
}

// Reads network and returns the peers connected to the network
func readNetwork() ([]string, error) {
	data, err := lockedfile.Read(filename)
	if err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	var ports []string
	for scanner.Scan() {
		text := scanner.Text()
		if text == port || text == "" {
			continue
		}
		ports = append(ports, text)
	}
	return ports, nil
}
