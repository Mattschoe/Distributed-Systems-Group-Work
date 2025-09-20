package main

import (
	tcp "tcp-simulator/internal"
)

func main() {
	SYN := make(chan struct{})
	ACK := make(chan struct{})
	connection := tcp.Request{SYN, ACK}
	go server(connection)
}

func server(tcp.Request) {

}
