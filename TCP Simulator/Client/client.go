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
	connection, err := net.Dial("udp", "localhost:6969")

	if err != nil {
		panic(err)
	}
	defer connection.Close() //If shit hits the fan, part 2: Electric bogaloo

	segment, err := establishConnectionToServer(connection)
	if err != nil {
		panic(err)
	}
	fmt.Println("Accepted connection with client on port", segment.DestinationPort)

	fmt.Println("Sending package...")

	sendWindow(segment, connection)
	readACK(segment, connection)
}

func sendWindow(segment *tcp.TCP, connection net.Conn) {
	input := make([]byte, 32)
	for i := 0; i < int(segment.WindowSize); i++ {
		binary.BigEndian.PutUint32(input, uint32(i))
		_, err := connection.Write(input)
		if err != nil {
			panic(err)
		}
	}
}

func readACK(segment *tcp.TCP, connection net.Conn) {
	fmt.Println("Receiving ACK's")
	output := make([]uint32, int(segment.WindowSize))
	seq := segment.Seq
	ackBytes := make([]byte, 32)
	for i := 0; i < int(segment.WindowSize); i++ {
		bytesReceived, err := connection.Read(ackBytes)
		if err != nil {
			panic(err)
		}
		if bytesReceived != len(ackBytes) {
			fmt.Errorf("expected %d bytes but received %d bytes", len(ackBytes), bytesReceived)
		}
		ackInt := binary.BigEndian.Uint32(ackBytes)
		if ackInt != seq+uint32(i) {
			fmt.Println("- expected:", seq+uint32(i), "but received", ackInt, "sending original segment...")
			input := make([]byte, 32)
			binary.BigEndian.PutUint32(input, seq+uint32(i))
			connection.Write(input)

			connection.Read(ackBytes)
			ackInt = binary.BigEndian.Uint32(ackBytes)
		}
		fmt.Println("- received ACK for:", ackInt)
		output[i] = ackInt
	}
	fmt.Println("Finished receiving ACK's for segment")
}

func establishConnectionToServer(connection net.Conn) (*tcp.TCP, error) {
	tcpHeaderSize := 20
	clientPort := uint16(6969)
	initialSeq := uint32(69)
	windowSize := uint16(1024)

	//Sends initial SYN segment
	synSeq := tcp.TCP{
		SourcePort:        clientPort,
		DestinationPort:   clientPort,
		Seq:               initialSeq,
		Ack:               0,
		OffsetAndReserved: 0,
		Flags:             tcp.SYN_FLAG,
		WindowSize:        windowSize,
		Checksum:          0,
		Urgent:            0,
	}
	fmt.Println("- Sending SYN with seq:", synSeq.Seq)
	_, err := connection.Write(synSeq.ToBinary())
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
	synAckSeq, err := tcp.ParseTCPSegment(buffer[:bytesReceived])
	if err != nil {
		return nil, err
	}
	if !synAckSeq.HasFlag(tcp.ACK_FLAG) || !synAckSeq.HasFlag(tcp.SYN_FLAG) || synAckSeq.Ack != synSeq.Seq+1 || synAckSeq.Seq == 0 {
		return nil, errors.New("invalid SYN-ACK segment returned from server")
	}
	fmt.Println("- SYN-ACK received from server, sending ACK")

	//Sends ACK segment
	ackSeq := tcp.TCP{
		SourcePort:      synAckSeq.DestinationPort,
		DestinationPort: synAckSeq.SourcePort,
		Seq:             synAckSeq.Seq + 1,
		Ack:             synAckSeq.Ack + 1,
		Flags:           tcp.ACK_FLAG,
		WindowSize:      windowSize,
	}

	_, err = connection.Write(ackSeq.ToBinary())
	if err != nil {
		return nil, fmt.Errorf("error sending ACK to server %d", err)
	}
	return synAckSeq, nil
}
