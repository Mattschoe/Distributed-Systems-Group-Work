package main

import (
	pb "Auction-House/grpc"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rogpeppe/go-internal/lockedfile"
	"google.golang.org/grpc"
)

var filename string
var port string
var ID2Auction map[int32]pb.Outcome

type auctionServer struct {
	pb.UnimplementedAuctionHouseServer
}

func main() {
	filename = filepath.Join(os.TempDir(), "LiveProcesses.txt")
	port = ":" + strconv.Itoa(rand.Intn(10_000))
	ID2Auction = make(map[int32]pb.Outcome)
	listener, err := net.Listen("tcp", port)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	err = subscribeToNetwork()
	if err != nil {
		panic(err)
	}

	go func() {
		server := grpc.NewServer()
		pb.RegisterAuctionHouseServer(server, &auctionServer{})
		err = server.Serve(listener)
		if err != nil {
			panic(err)
		}
	}()

	log.Printf("Process is running on port %s. \n - Use '.Quit' to quit. \n - Use '.Sell <product> <timeframe>' to start an auction. \n - Use '.Bid <auction_id> <amount>' to bid on a auction", port)
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		input := strings.Split(scanner.Text(), " ")

		if len(input) == 1 {
			if input[0] == ".Quit" || input[0] == ".quit" {
				log.Printf("Shutting down process on port %v... \n", port)
				err := unsubscribeFromNetwork()
				if err != nil {
					log.Printf("Error shutting down process with error: %v \n", err)
				}
				if err := listener.Close(); err != nil {
					log.Printf("Error shutting down listener: %v \n", err)
				}
				os.Exit(0)
			}
		} else if len(input) == 3 {
			if input[0] == ".Sell" || input[0] == ".sell" {
				product := input[1]
				timeframe, err := strconv.Atoi(input[2])
				if err != nil {
					log.Printf("Error parsing timeframe: %v \n", err)
					continue
				}
				auctionStarted, auctionID, err := startAuction(product, timeframe)
				if err != nil {
					log.Printf("Error starting auction: %v \n", err)
					continue
				}

				if auctionStarted {
					go func() {
						fmt.Printf("Auction has started and will end in %d seconds.\n", timeframe)
						time.Sleep(time.Duration(timeframe) * time.Second)
						err = sendAuctionResult(auctionID)
						if err != nil {
							fmt.Println("Error sending auction result!")
						}
						fmt.Printf("Auction has finished, winner is:\n")

					}()
				} else {
					fmt.Println("Auction failed, not enough acknowledgements.")
				}
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
			fmt.Printf("Error connecting to client at port %s: %v", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)

		ack, err := client.Bid(context.Background(), &pb.BidInfo{
			AuctionID: int32(auctionID),
			ProcessID: int32(processID),
			Amount:    int32(amount),
		})
		if err != nil {
			return err
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
		ID2Auction[int32(auctionID)] = pb.Outcome{
			Status:     pb.Outcome_Running,
			AuctionID:  int32(auctionID),
			WinnerID:   int32(processID),
			CurrentBid: int32(amount),
		}
	}
	return nil
}

func (server *auctionServer) Bid(ctx context.Context, info *pb.BidInfo) (*pb.BidAcknowledgement, error) {
	outcome, ok := ID2Auction[info.AuctionID]
	if !ok {
		//TODO Fix
		fmt.Printf("ARGH auctionen eksisterer ikke på min process, jeg skal bede om resultat her!")
		return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_Fail}, nil
	}

	fmt.Printf("Process: %d, bid on auction: %d, for %dkr \n", info.ProcessID, info.AuctionID, info.Amount)
	if outcome.CurrentBid < info.Amount {
		ID2Auction[info.AuctionID] = pb.Outcome{
			Status:     outcome.Status,
			WinnerID:   info.ProcessID,
			CurrentBid: info.Amount,
			AuctionID:  outcome.AuctionID,
		}
		return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_Success}, nil
	}
	fmt.Println("Bid too low!")
	return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_TooLow}, nil
}

func sendAuctionResult(auctionID int32) error {
	outcome, ok := ID2Auction[auctionID]
	if !ok {
		return errors.New("cant find the auction")
	}

	ports, err := readNetwork()
	if err != nil {
		fmt.Printf("Error reading network ports: %v", err)
		return err
	}

	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)

		_, err = client.Result(context.Background(), &pb.Outcome{
			Status:     pb.Outcome_Finished,
			AuctionID:  auctionID,
			WinnerID:   outcome.WinnerID,
			CurrentBid: outcome.CurrentBid,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func (server *auctionServer) Result(ctx context.Context, outcome *pb.Outcome) (*pb.Empty, error) {
	fmt.Printf("Auction result: %v\n", outcome)
	ID2Auction[outcome.AuctionID] = *outcome
	return nil, nil
}

// Returns true and auctionID if success and an auction has begun and false if a failure occured (like not enough acks)
func startAuction(product string, timeframe int) (bool, int32, error) {
	ports, err := readNetwork()
	if err != nil {
		fmt.Printf("Error reading network ports: %v \n", err)
		return false, -1, err
	}

	var acks []pb.SellAcknowledgement_Status
	auctionID := int32(rand.Intn(10_000))

	ID2Auction[auctionID] = pb.Outcome{
		Status:     pb.Outcome_Running,
		WinnerID:   -1,
		CurrentBid: -1,
		AuctionID:  auctionID,
	}

	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v \n", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)

		ack, err := client.Sell(context.Background(), &pb.Auction{
			ID:        auctionID,
			Product:   product,
			Timeframe: int32(timeframe),
		})
		if err != nil {
			fmt.Printf("Error starting auction: %v \n", err)
			continue
		}
		acks = append(acks, ack.Status)
	}

	var success int
	for _, ack := range acks {
		if ack == pb.SellAcknowledgement_Success {
			success++
		}
	}
	if success >= len(ports) {
		return true, auctionID, nil
	}
	return false, -1, nil
}

func (server *auctionServer) Sell(ctx context.Context, auction *pb.Auction) (*pb.SellAcknowledgement, error) {
	fmt.Printf("A auction has started! Auction n. %d is selling: '%s' at timeframe: %d \n", int(auction.ID), auction.Product, int(auction.Timeframe))
	ID2Auction[auction.ID] = pb.Outcome{
		Status:     pb.Outcome_Running,
		WinnerID:   -1,
		CurrentBid: -1,
		AuctionID:  auction.ID,
	}

	//Timeout for Result
	go func() {
		waitTime := int(auction.Timeframe) + rand.Intn(20) //Random so everyone doesn't start yapping at once
		time.Sleep(time.Duration(waitTime) * time.Second)

		status := ID2Auction[auction.ID].Status
		if status != pb.Outcome_Finished {
			holdQuorum(auction.ID)
		}
	}()

	return &pb.SellAcknowledgement{Status: pb.SellAcknowledgement_Success}, nil
}

// Holds a quorum agreeing to what the outcome of the auction should be
func holdQuorum(auctionID int32) {
	fmt.Println("Hey i think the auctioneer is dead, im holding a Quorum!")

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

	//Stores this process idea of the outcome
	outcome := ID2Auction[auctionID]
	fmt.Printf("DEBUG: My outcome: %v", outcome)
	outcome2Amount[outcomeKey{
		Status:     pb.Outcome_Finished,
		WinnerID:   outcome.WinnerID,
		AuctionID:  outcome.AuctionID,
		CurrentBid: outcome.CurrentBid,
	}]++

	//Gets outcome from every other port
	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v \n", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)
		clientOutcome, err := client.Quorum(context.Background(), &outcome)
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
		fmt.Printf("Outcome: %v | amount: %d | highest: %d\n", outcome, amount, highest)
		if amount > highest {
			highest = amount
			winner = outcome
		}
	}
	newOutcome := pb.Outcome{
		Status:     pb.Outcome_Finished,
		WinnerID:   winner.WinnerID,
		CurrentBid: winner.CurrentBid,
		AuctionID:  winner.AuctionID,
	}
	fmt.Printf("New outcome: %v \n", newOutcome)
	ID2Auction[auctionID] = newOutcome
	fmt.Printf("Quorum is finished and we agreed on the result: %v \n", ID2Auction[auctionID])
	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v \n", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)
		client.Result(context.Background(), &newOutcome)
	}
}

func (server *auctionServer) Quorum(ctx context.Context, outcome *pb.Outcome) (*pb.Outcome, error) {
	fmt.Println("A process thinks a auction is dead, i'll send my outcome!")
	myOutcome := ID2Auction[outcome.AuctionID]
	return &myOutcome, nil
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

func subscribeToNetwork() error {
	file, err := lockedfile.Edit(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	_, err = file.WriteString(port + "\n")
	if err != nil {
		return err
	}

	return nil
}

func unsubscribeFromNetwork() error {
	file, err := lockedfile.Edit(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var newLines []string
	for scanner.Scan() {
		text := scanner.Text()
		if text == port {
			continue
		}
		newLines = append(newLines, text)
	}

	err = scanner.Err()
	if err != nil {
		return err
	}

	err = file.Truncate(0)
	if err != nil {
		return err
	}

	_, err = file.Seek(0, 0)
	if err != nil {
		return err
	}

	for _, line := range newLines {
		_, err = file.WriteString(line + "\n")
		if err != nil {
			return err
		}
	}
	return nil
}
