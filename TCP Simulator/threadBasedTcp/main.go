package main

import (
	"fmt"
	tcp "tcp-simulator/internal"
)

func main() {
	// shared channels
	req := tcp.Request{
		SYN: make(chan struct{}),
		ACK: make(chan struct{}),
	}

	go server(req) // start server goroutine
	go client(req) // start client goroutine

	select {} // block forever (or use sync.WaitGroup if you want exit)
}

func server(r tcp.Request) {
	fmt.Println("Server waiting for SYN...")
	<-r.SYN // wait for client SYN
	fmt.Println("Server got SYN, sending ACK")
	r.ACK <- struct{}{}
}

func client(r tcp.Request) {
	fmt.Println("Client sending SYN")
	r.SYN <- struct{}{}
	fmt.Println("Client waiting for ACK...")
	<-r.ACK
	fmt.Println("Client got ACK, connection established!")
}
