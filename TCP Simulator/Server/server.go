package main

import (
	tcp "TCP_Simulator/internal"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"time"
)

func main() {
	listener, err := net.ListenPacket("udp", ":6969")
	fmt.Println("Listening on :6969")

	if err != nil {
		panic(err)
	}
	defer listener.Close() //If shit hits the fan

	segment, address, err := AcceptConnection(listener)
	if err != nil {
		panic(err)
	}
	fmt.Println("Accepted connection with client on port", segment.DestinationPort, "!")
	println()

	output := readWindow(segment, listener)
	returnACK(output, listener, segment, address)
}

func readWindow(segment *tcp.TCP, listener net.PacketConn) []uint32 {
	fmt.Println("Waiting to receive packages...")
	output := make([]uint32, int(segment.WindowSize))
	inputBytes := make([]byte, 32)
	for i := 0; i < int(segment.WindowSize); i++ {
		bytesReceived, _, err := listener.ReadFrom(inputBytes)
		if err != nil {
			panic(err)
		}
		if bytesReceived != len(inputBytes) {
			fmt.Errorf("expected %d bytes but received %d bytes", len(inputBytes), bytesReceived)
		}
		inputInt := binary.BigEndian.Uint32(inputBytes)
		output[inputInt] = inputInt
	}
	return output
}

// Returns acknowledgement for each segment received
func returnACK(output []uint32, listener net.PacketConn, segment *tcp.TCP, address net.Addr) {
	fmt.Println("- Returning ACK")
	ack := make([]byte, 32)
	output[4] = 0 //Force data transfer error to showcase segment request
	for i := 0; i < int(segment.WindowSize); i++ {
		if int(output[i]) == i {
			binary.BigEndian.PutUint32(ack, segment.Seq+output[i])
			_, err := listener.WriteTo(ack, address)
			if err != nil {
				panic(err)
			}
		} else {
			fmt.Println("- Expected data not correct, requesting correct package...")
			inputBytes := make([]byte, 32)

			binary.BigEndian.PutUint32(ack, 0)
			_, err := listener.WriteTo(ack, address) //Sends 0 as error (not -1 because accidently made uint)
			if err != nil {
				panic(err)
			}

			_, _, err = listener.ReadFrom(inputBytes) //Waits for correct package
			if err != nil {
				panic(err)
			}
			inputInt := binary.BigEndian.Uint32(inputBytes)
			output[i] = inputInt
			fmt.Println("- Received:", inputInt, "from client, sending ack...")
			binary.BigEndian.PutUint32(ack, inputInt)
			_, err = listener.WriteTo(ack, address)
		}
	}
	fmt.Println("- Finished sending ACK for segment")
}

func AcceptConnection(listener net.PacketConn) (seg *tcp.TCP, address net.Addr, err error) {
	tcpHeaderSize := 20 //20 Since thats the size of the TCPHeader
	buffer := make([]byte, tcpHeaderSize)

	//Reads initial SYN segment
	bytesReceived, address, err := listener.ReadFrom(buffer)
	if err != nil {
		return nil, nil, err
	}
	if bytesReceived != tcpHeaderSize {
		fmt.Println("Expected", tcpHeaderSize, "bytes, received", bytesReceived)
	}

	//parses Clients SYN segment
	synSegment, err := tcp.ParseTCPSegment(buffer[:bytesReceived])
	if err != nil {
		return nil, nil, err
	}
	if !synSegment.HasFlag(tcp.SYN_FLAG) {
		return nil, nil, errors.New("SYN_FLAG not set on initial segment recieved")
	}
	fmt.Println("- Received segment with SYN_FLAG, sending acknowledgement..")

	//Sends SYN-ACK response
	synAckTCP := tcp.TCP{
		Ack:        synSegment.Seq + 1,
		Seq:        420,
		Flags:      tcp.SYN_FLAG | tcp.ACK_FLAG,
		WindowSize: uint16(10),
	}
	_, err = listener.WriteTo(synAckTCP.ToBinary(), address)
	if err != nil {
		return nil, nil, err
	}

	//Waits for Client ACK
	fmt.Println("- Sent SYN-ACK acknowledgement, waiting for acknowledgement ACK back...")
	err = listener.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		return nil, nil, err
	}
	ackBytesReceived, _, err := listener.ReadFrom(buffer)
	if err != nil {
		return nil, nil, err
	}
	if ackBytesReceived != tcpHeaderSize {
		return nil, nil, fmt.Errorf("expected", tcpHeaderSize, "bytes, received", ackBytesReceived)
	}

	//Parses Client ACK segment
	ackSegment, err := tcp.ParseTCPSegment(buffer[:tcpHeaderSize])
	if err != nil {
		return nil, nil, err
	}

	if !ackSegment.HasFlag(tcp.ACK_FLAG) || ackSegment.HasFlag(tcp.SYN_FLAG) || ackSegment.Seq != synAckTCP.Seq+1 || ackSegment.Ack != synAckTCP.Ack+1 {
		return nil, nil, fmt.Errorf("invalid ACK segment returned from client, 3-way handshake failed")
	}
	return &synAckTCP, address, nil
}
