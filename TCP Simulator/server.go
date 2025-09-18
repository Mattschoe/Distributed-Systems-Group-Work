package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
)

func main() {
	listener, err := net.Listen("tcp", "localhost:6969")

	if err != nil {
		panic(err)
	}
	defer listener.Close() //If shit hits the fan

	for {
		connection, err := listener.Accept()
		if err != nil {
			panic(err)
		}

		bufferSize := 20
		readBuffer := make([]byte, bufferSize) //Size 20 since that's what TCP uses.
		bytesReceived, _ := connection.Read(readBuffer)
		if bytesReceived != bufferSize {
			fmt.Println("Expected", bufferSize, "bytes, received", bytesReceived)
			continue
		}

		var tcpHeader TCP
		buffer := bytes.NewReader(readBuffer[:bytesReceived])
		readErr := binary.Read(buffer, binary.BigEndian, &tcpHeader)
		if readErr != nil {
			panic(readErr)
		}

		fmt.Println(tcpHeader.Seq)
	}
}
