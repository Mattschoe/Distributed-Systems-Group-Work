package main

import (
	"fmt"
	tcp "tcp-simulator/internal"
)

func main() {
	// shared channels
	socC := make(chan tcp.Packet)
	socS := make(chan tcp.Packet)

	go server(socS, socC) // start server goroutine
	go client(socC, socS) // start client goroutine

	select {} // block forever (or use sync.WaitGroup if you want exit)
}

func server(soc chan tcp.Packet, cli chan tcp.Packet) {
	fmt.Println("Server waiting for SYN...")
	fmt.Println(<-soc)
	fmt.Printf("Server got SYN, sending SYN-ACK ")
	cli <- tcp.Packet{SYN: true, ACK: true}
	fmt.Println("Server waiting for ACK")
	fmt.Println(<-soc)
	fmt.Println("Server got ACK, connection established!")
}

func client(soc chan tcp.Packet, ser chan tcp.Packet) {
	fmt.Println("Client sending SYN")
	ser <- tcp.Packet{SYN: true}
	fmt.Println("Client waiting for SYN-ACK")
	fmt.Println(<-soc)
	fmt.Println("Client got SYN-ACK, connection established!")
	ser <- tcp.Packet{ACK: true}
}
