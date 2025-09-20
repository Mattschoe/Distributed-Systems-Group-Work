package main

import (
	"bytes"
	"encoding/binary"
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

	connectionEstablished, err := establishConnectionToServer(connection)
	if err != nil {
		panic(err)
	}
	if !connectionEstablished {
		panic("Could not establish connection to server")
	}
	fmt.Println("Connection established successfully on Client side")
}

func establishConnectionToServer(connection net.Conn) (connectionEstablished bool, err error) {
	tcpHeader := TCP{
		SourcePort:        6969,
		DestinationPort:   6969,
		Seq:               69,
		Ack:               0,
		OffsetAndReserved: 0,
		Flags:             SYN_FLAG,
		WindowSize:        1024,
		Checksum:          0,
		Urgent:            0,
	}
	fmt.Println("- Sending SYN with seq:", tcpHeader.Seq)
	connection.Write(tcpHeader.toBinary())

	//Waits and Reads for Server ACK-SYN
	readDeadlineErr := connection.SetReadDeadline(time.Now().Add(2 * time.Second))
	if readDeadlineErr != nil {
		return false, readDeadlineErr
	}
	bufferSize := 20
	readBuffer := make([]byte, bufferSize) //Size 20 since that's what TCP uses without options added.
	fmt.Println("- Waiting for ACK-SYN back")
	bytesReceived, err := connection.Read(readBuffer)
	if bytesReceived != bufferSize {
		return false, fmt.Errorf("Expected to receive %d bytes, received %d", bufferSize, bytesReceived)
	}
	if err != nil {
		panic(err)
	}

	//Converts to TCP
	var segment TCP
	buffer := bytes.NewReader(readBuffer[:bytesReceived])
	readErr := binary.Read(buffer, binary.BigEndian, &segment)
	if readErr != nil {
		panic(readErr)
	}

	if segment.hasFlag(ACK_FLAG) && segment.hasFlag(SYN_FLAG) && segment.Ack == tcpHeader.Seq+1 && segment.Seq != 0 {
		fmt.Println("- ACK-SYN correctly established, sending ACK back")

		ackTCP := TCP{
			SourcePort:        segment.SourcePort,
			DestinationPort:   segment.DestinationPort,
			Seq:               segment.Seq + 1,
			Ack:               segment.Ack + 1,
			OffsetAndReserved: 0,
			Flags:             ACK_FLAG,
			WindowSize:        1024,
			Checksum:          0,
			Urgent:            0,
		}
		connection.Write(ackTCP.toBinary())
		return true, nil
	}
	return false, nil
}
