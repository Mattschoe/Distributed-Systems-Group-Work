package main

import (
	"encoding/binary"
	"net"
)

func main() {
	connection, err := net.Dial("tcp", "localhost:6969")

	if err != nil {
		panic(err)
	}
	defer connection.Close() //If shit hits the fan, part 2: Electric bogaloo

	seq := make([]byte, 4)
	binary.BigEndian.PutUint32(seq, 300)
	connection.Write([]byte{1, 2, 3, 4, 5})
}
