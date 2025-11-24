package main

import (
	pb "Auction-House/grpc"
	"bufio"
	"context"
	"fmt"
	"io"
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

type auctionServer struct {
	pb.UnimplementedAuctionHouseServer
}

func main() {
	filename = filepath.Join(os.TempDir(), "LiveProcesses.txt")
	port = ":" + strconv.Itoa(rand.Intn(10_000))
	ID2Auction = make(map[int32]*pb.Outcome)
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

	log.Printf("Process is running on port %s. \n - Use '.Quit' to quit. \n", port)
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
		}
	}
}

func (server *auctionServer) Bid(ctx context.Context, info *pb.BidInfo) (*pb.BidAcknowledgement, error) {
	auctionMutex.Lock()
	defer auctionMutex.Unlock()

	outcome, ok := ID2Auction[info.AuctionID]
	if !ok {
		//TODO Fix
		fmt.Printf("ARGH auctionen eksisterer ikke på min process, jeg skal bede om resultat her!")
		return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_Fail}, nil
	}

	// Check if auction is already finished
	if outcome.GetStatus() == pb.Outcome_Finished {
		fmt.Printf("Auction %d is already finished. Rejecting bid from process %d.\n", info.AuctionID, info.ProcessID)
		return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_Fail}, nil
	}

	fmt.Printf("Process: %d, bid on auction: %d, for %dkr \n", info.ProcessID, info.AuctionID, info.Amount)
	if outcome.GetCurrentBid() < info.Amount {
		ID2Auction[info.AuctionID] = &pb.Outcome{
			Status:     outcome.GetStatus(),
			WinnerID:   info.ProcessID,
			CurrentBid: info.Amount,
			AuctionID:  outcome.GetAuctionID(),
		}
		return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_Success}, nil
	}
	fmt.Println("Bid too low!")
	return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_TooLow}, nil
}

func (server *auctionServer) Sell(ctx context.Context, auction *pb.Auction) (*pb.Outcome, error) {
	fmt.Printf("A auction has started! Auction n. %d is selling: '%s' at timeframe: %d \n", int(auction.ID), auction.Product, int(auction.Timeframe))
	
	auctionMutex.Lock()
	ID2Auction[auction.ID] = &pb.Outcome{
		Status:     pb.Outcome_Running,
		WinnerID:   -1,
		CurrentBid: -1,
		AuctionID:  auction.ID,
	}
	auctionMutex.Unlock()

	timeframe := time.Duration(auction.Timeframe) * time.Second

	select {
	case <-time.After(timeframe):
		{
			auctionMutex.Lock()
			currentStatus := ID2Auction[auction.ID]
			ID2Auction[auction.ID] = &pb.Outcome{
				Status:     pb.Outcome_Finished,
				WinnerID:   currentStatus.GetWinnerID(),
				CurrentBid: currentStatus.GetCurrentBid(),
				AuctionID:  auction.ID,
			}

			result := &pb.Outcome{
				Status:     pb.Outcome_Finished,
				WinnerID:   currentStatus.GetWinnerID(),
				CurrentBid: currentStatus.GetCurrentBid(),
				AuctionID:  auction.ID,
			}
			auctionMutex.Unlock()
			
			fmt.Printf("Auction is over, sending the result: { %v } \n", result)
			return result, nil
		}
	case <-ctx.Done():
		fmt.Println("Context cancelled")
		return nil, ctx.Err()
	}
}

func (server *auctionServer) Status(ctx context.Context, empty *pb.Empty) (*pb.Auctions, error) {
	auctionMutex.RLock()
	defer auctionMutex.RUnlock()
	
	var auctions []*pb.Outcome
	for auctionID := range ID2Auction {
		auction := ID2Auction[auctionID]
		// Create a copy to avoid lock issues
		auctionCopy := &pb.Outcome{
			Status:     auction.GetStatus(),
			WinnerID:   auction.GetWinnerID(),
			CurrentBid: auction.GetCurrentBid(),
			AuctionID:  auction.GetAuctionID(),
		}
		auctions = append(auctions, auctionCopy)
	}
	return &pb.Auctions{
		Auctions: auctions,
	}, nil
}

func (server *auctionServer) Result(ctx context.Context, outcome *pb.Outcome) (*pb.Outcome, error) {
	auctionMutex.RLock()
	myOutcome := ID2Auction[outcome.AuctionID]
	auctionMutex.RUnlock()
	
	return &pb.Outcome{
		Status:     myOutcome.GetStatus(),
		WinnerID:   myOutcome.GetWinnerID(),
		CurrentBid: myOutcome.GetCurrentBid(),
		AuctionID:  myOutcome.GetAuctionID(),
	}, nil
}

func (server *auctionServer) UpdateResult(ctx context.Context, outcome *pb.Outcome) (*pb.Empty, error) {
	auctionMutex.Lock()
	ID2Auction[outcome.AuctionID] = &pb.Outcome{
		Status:     outcome.Status,
		WinnerID:   outcome.WinnerID,
		CurrentBid: outcome.CurrentBid,
		AuctionID:  outcome.AuctionID,
	}
	auctionMutex.Unlock()
	return &pb.Empty{}, nil
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
