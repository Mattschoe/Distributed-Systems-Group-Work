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

func (server *auctionServer) Sell(ctx context.Context, auction *pb.Auction) (*pb.Outcome, error) {
	fmt.Printf("A auction has started! Auction n. %d is selling: '%s' at timeframe: %d \n", int(auction.ID), auction.Product, int(auction.Timeframe))
	ID2Auction[auction.ID] = pb.Outcome{
		Status:     pb.Outcome_Running,
		WinnerID:   -1,
		CurrentBid: -1,
		AuctionID:  auction.ID,
	}

	timeframe := time.Duration(auction.Timeframe) * time.Second

	select {
	case <-time.After(timeframe):
		{
			status := ID2Auction[auction.ID]
			ID2Auction[auction.ID] = pb.Outcome{
				Status:     pb.Outcome_Finished,
				WinnerID:   status.WinnerID,
				CurrentBid: status.CurrentBid,
				AuctionID:  auction.ID,
			}

			result := ID2Auction[auction.ID]
			fmt.Printf("Auction is over, sending the result: { %v } \n", &result)
			return &result, nil
		}
	case <-ctx.Done():
		fmt.Println("Context cancelled")
		return nil, ctx.Err()
	}
}

func (server *auctionServer) Status(ctx context.Context, empty *pb.Empty) (*pb.Auctions, error) {
	var auctions []*pb.Outcome
	for _, auction := range ID2Auction {
		auctions = append(auctions, &auction)
	}
	return &pb.Auctions{
		Auctions: auctions,
	}, nil
}

func (server *auctionServer) Result(ctx context.Context, outcome *pb.Outcome) (*pb.Outcome, error) {
	myOutcome := ID2Auction[outcome.AuctionID]
	return &myOutcome, nil
}

func (server *auctionServer) UpdateResult(ctx context.Context, outcome *pb.Outcome) (*pb.Empty, error) {
	ID2Auction[outcome.AuctionID] = *outcome
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
