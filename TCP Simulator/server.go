package main

import (
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

func AcceptConnection(listener net.PacketConn) (seg *TCP, err error) {
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
	synSegment, err := parseTCPSegment(buffer[:bytesReceived])
	if err != nil {
		return nil, err
	}
	if !synSegment.hasFlag(SYN_FLAG) {
		return nil, errors.New("SYN_FLAG not set on initial segment recieved")
	}
	fmt.Println("- Received segment with SYN_FLAG, sending acknowledgement..")

	//Sends SYN-ACK response
	synAckTCP := TCP{
		Ack:   synSegment.Seq + 1,
		Seq:   420,
		Flags: SYN_FLAG | ACK_FLAG,
	}
	_, err = listener.WriteTo(synAckTCP.toBinary(), address)
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
	ackSegment, err := parseTCPSegment(buffer[:tcpHeaderSize])
	if err != nil {
		return nil, err
	}

	if !ackSegment.hasFlag(ACK_FLAG) || ackSegment.hasFlag(SYN_FLAG) || ackSegment.Seq != synAckTCP.Seq+1 || ackSegment.Ack != synAckTCP.Ack+1 {
		return nil, fmt.Errorf("invalid ACK segment returned from client, 3-way handshake failed")
	}
	return &synAckTCP, nil
}
