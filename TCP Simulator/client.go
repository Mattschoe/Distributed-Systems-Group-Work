package main

import (
	"bytes"
	"encoding/binary"
	"net"
)

func main() {
	connection, err := net.Dial("tcp", "localhost:6969")

	if err != nil {
		panic(err)
	}
	defer connection.Close() //If shit hits the fan, part 2: Electric bogaloo

	tcpHeader := TCP{
		SourcePort:        8080,
		DestinationPort:   8080,
		Seq:               69,
		Ack:               0,
		OffsetAndReserved: 0,
		Flags:             SYN_FLAG,
		WindowSize:        1024,
		Checksum:          0,
		Urgent:            0,
	}
	var tcpBuffer bytes.Buffer
	binary.Write(&tcpBuffer, binary.BigEndian, tcpHeader)
	connection.Write(tcpBuffer.Bytes())
}
