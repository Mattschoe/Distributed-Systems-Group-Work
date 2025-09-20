package main

import (
	tcp "TCP_Simulator/internal"
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

	segment, err := AcceptConnection(listener)
	if err != nil {
		panic(err)
	}
	fmt.Println("Accepted connection with client", segment)
}

func AcceptConnection(listener net.PacketConn) (seg *tcp.TCP, err error) {
	tcpHeaderSize := 20 //20 Since thats the size of the TCPHeader
	buffer := make([]byte, tcpHeaderSize)

	//Reads initial SYN segment
	bytesReceived, address, err := listener.ReadFrom(buffer)
	if err != nil {
		return nil, err
	}
	if bytesReceived != tcpHeaderSize {
		fmt.Println("Expected", tcpHeaderSize, "bytes, received", bytesReceived)
	}

	//parses Clients SYN segment
	synSegment, err := tcp.ParseTCPSegment(buffer[:bytesReceived])
	if err != nil {
		return nil, err
	}
	if !synSegment.HasFlag(tcp.SYN_FLAG) {
		return nil, errors.New("SYN_FLAG not set on initial segment recieved")
	}
	fmt.Println("- Received segment with SYN_FLAG, sending acknowledgement..")

	//Sends SYN-ACK response
	synAckTCP := tcp.TCP{
		Ack:   synSegment.Seq + 1,
		Seq:   420,
		Flags: tcp.SYN_FLAG | tcp.ACK_FLAG,
	}
	_, err = listener.WriteTo(synAckTCP.ToBinary(), address)
	if err != nil {
		return nil, err
	}

	//Waits for Client ACK
	fmt.Println("- Sent SYN-ACK acknowledgement, waiting for acknowledgement ACK back...")
	err = listener.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		return nil, err
	}
	ackBytesReceived, _, err := listener.ReadFrom(buffer)
	if err != nil {
		return nil, err
	}
	if ackBytesReceived != tcpHeaderSize {
		return nil, fmt.Errorf("expected", tcpHeaderSize, "bytes, received", ackBytesReceived)
	}

	//Parses Client ACK segment
	ackSegment, err := tcp.ParseTCPSegment(buffer[:tcpHeaderSize])
	if err != nil {
		return nil, err
	}

	if !ackSegment.HasFlag(tcp.ACK_FLAG) || ackSegment.HasFlag(tcp.SYN_FLAG) || ackSegment.Seq != synAckTCP.Seq+1 || ackSegment.Ack != synAckTCP.Ack+1 {
		return nil, fmt.Errorf("invalid ACK segment returned from client, 3-way handshake failed")
	}
	return &synAckTCP, nil
}
