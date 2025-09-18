package main

import (
	"encoding/binary"
	"fmt"
	"net"
)

type TCP struct {
}

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

		readBuffer := make([]byte, 4) //Size 4 since that's what TCP uses.
		bytesReceived, _ := connection.Read(readBuffer)
		if bytesReceived != 4 {
			fmt.Println("Expected 4 bytes, received", bytesReceived)
			continue
		}
		fmt.Println(binary.BigEndian.Uint32(readBuffer))
	}
}
