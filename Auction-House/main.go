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

	for _, port := range ports {
		conn, err := grpc.NewClient("localhost"+port, grpc.WithInsecure(), grpc.WithBlock())
		if err != nil {
			fmt.Printf("Error connecting to client at port %s: %v", port, err)
			continue
		}
		client := pb.NewAuctionHouseClient(conn)
		processID, err := strconv.Atoi(strings.Split(port, ":")[1])
		if err != nil {
			return err
		}
		ack, err := client.Bid(context.Background(), &pb.BidInfo{
			AuctionID: int32(auctionID),
			ProcessID: int32(processID),
			Amount:    int32(amount),
		})
		if err != nil {
			return err
		}

		if ack.Status == pb.BidAcknowledgement_Fail {
			fmt.Println("Sending bid failed, try again!")
			return nil
		}
	}

	fmt.Println("Sending bid succeeded!")
	return nil
}

func (server *auctionServer) Bid(ctx context.Context, info *pb.BidInfo) (*pb.BidAcknowledgement, error) {
	outcome, ok := ID2Auction[info.AuctionID]
	if !ok {
		//TODO Fix
		fmt.Printf("ARGH auctionen eksisterer ikke på min process, jeg skal bede om resultat her!")
		return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_Fail}, nil
	}

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
	return &pb.BidAcknowledgement{Status: pb.BidAcknowledgement_Success}, nil
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
	return &pb.SellAcknowledgement{Status: pb.SellAcknowledgement_Success}, nil
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
