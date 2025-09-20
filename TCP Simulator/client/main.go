package main

import (
	tcp "tcp-simulator/internal"
)

func main() {
	SYN := make(chan struct{})
	ACK := make(chan struct{})
	connection := tcp.Request{SYN, ACK}
	go client(connection)
}

func client(request tcp.Request) {
	establishConnection(request)

}

func establishConnection(tcp.Request) {

}
