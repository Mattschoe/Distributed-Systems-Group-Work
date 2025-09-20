package main

import (
	"errors"
	"fmt"
	"net"
	"time"
)

func main() {
	connection, err := net.Dial("udp", "localhost:6969")

	if err != nil {
		panic(err)
	}
	defer connection.Close() //If shit hits the fan, part 2: Electric bogaloo

	_, err = establishConnectionToServer(connection)
	if err != nil {
		panic(err)
	}
	fmt.Println("Connection established successfully on Client side")
}

func establishConnectionToServer(connection net.Conn) (*TCP, error) {
	tcpHeaderSize := 20
	clientPort := uint16(6969)
	initialSeq := uint32(69)
	windowSize := uint16(1024)

	//Sends initial SYN segment
	synSeq := TCP{
		SourcePort:        clientPort,
		DestinationPort:   clientPort,
		Seq:               initialSeq,
		Ack:               0,
		OffsetAndReserved: 0,
		Flags:             SYN_FLAG,
		WindowSize:        windowSize,
		Checksum:          0,
		Urgent:            0,
	}
	fmt.Println("- Sending SYN with seq:", synSeq.Seq)
	_, err := connection.Write(synSeq.toBinary())
	if err != nil {
		return nil, err
	}

	//Waits for SYN-ACK response from server
	fmt.Println("- Waiting for SYN-ACK response from server")
	err = connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	if err != nil {
		return nil, err
	}
	buffer := make([]byte, tcpHeaderSize)
	bytesReceived, err := connection.Read(buffer)
	if err != nil {
		return nil, err
	}
	if bytesReceived != tcpHeaderSize {
		return nil, fmt.Errorf("expected", tcpHeaderSize, "bytes, received", bytesReceived)
	}

	//Parses and validates SYN-ACK segment
	synAckSeq, err := parseTCPSegment(buffer[:bytesReceived])
	if err != nil {
		return nil, err
	}
	if !synAckSeq.hasFlag(ACK_FLAG) || !synAckSeq.hasFlag(SYN_FLAG) || synAckSeq.Ack != synSeq.Seq+1 || synAckSeq.Seq == 0 {
		return nil, errors.New("invalid SYN-ACK segment returned from server")
	}
	fmt.Println("- SYN-ACK received from server, sending ACK")

	//Sends ACK segment
	ackSeq := TCP{
		SourcePort:      synAckSeq.DestinationPort,
		DestinationPort: synAckSeq.SourcePort,
		Seq:             synAckSeq.Seq + 1,
		Ack:             synAckSeq.Ack + 1,
		Flags:           ACK_FLAG,
		WindowSize:      windowSize,
	}

	_, err = connection.Write(ackSeq.toBinary())
	if err != nil {
		return nil, fmt.Errorf("error sending ACK to server %d", err)
	}
	return synAckSeq, nil
}
