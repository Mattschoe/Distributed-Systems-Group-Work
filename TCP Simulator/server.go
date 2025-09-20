package main

import (
	"bytes"
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

	segment, err := AcceptConnection(listener)
	if err != nil {
		panic(err)
	}
	fmt.Println("Accepted connection with client", segment)
}

func AcceptConnection(listener net.PacketConn) (seg *TCP, err error) {
	bufferSize := 20
	readBuffer := make([]byte, bufferSize) //Size 20 since that's what TCP uses without options added.
	bytesReceived, address, err := listener.ReadFrom(readBuffer)
	if bytesReceived != bufferSize {
		fmt.Println("Expected", bufferSize, "bytes, received", bytesReceived)
	}
	if err != nil {
		panic(err)
	}

	//Translates segment to TCP
	var segment TCP
	buffer := bytes.NewReader(readBuffer[:bytesReceived])
	readErr := binary.Read(buffer, binary.BigEndian, &segment)
	if readErr != nil {
		panic(readErr)
	}

	if segment.hasFlag(SYN_FLAG) {
		fmt.Println("- Received segment with SYN_FLAG, sending acknowledgement")
		seqAckTCP := segment
		seqAckTCP.Ack = segment.Seq + 1
		seqAckTCP.Seq = 420
		seqAckTCP.Flags = SYN_FLAG | ACK_FLAG

		_, writeErr := listener.WriteTo(seqAckTCP.toBinary(), address) //sending acknowledgement back
		if writeErr != nil {
			panic(writeErr)
		}

		//Waits for Client ACK
		readDeadlineErr := listener.SetReadDeadline(time.Now().Add(2 * time.Second))
		if readDeadlineErr != nil {
			return nil, readDeadlineErr
		}

		ackBytesReceived, _, ackErr := listener.ReadFrom(readBuffer)
		if ackBytesReceived != bufferSize {
			fmt.Println("Expected", bufferSize, "bytes, received", ackBytesReceived)
		}
		if ackErr != nil {
			panic(ackErr)
		}

		var ackSegment TCP
		ackBuffer := bytes.NewReader(readBuffer[:ackBytesReceived])
		readErr2 := binary.Read(ackBuffer, binary.BigEndian, &ackSegment)
		if readErr2 != nil {
			panic(readErr2)
		}

		if ackSegment.hasFlag(ACK_FLAG) && !ackSegment.hasFlag(SYN_FLAG) && ackSegment.Ack == seqAckTCP.Ack+1 && ackSegment.Seq == seqAckTCP.Seq+1 {
			return &ackSegment, nil
		}
	}
	return nil, errors.New("error establishing 3-way handshake")
}
