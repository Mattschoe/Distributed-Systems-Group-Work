package main

import (
	"encoding/json"
	"log"
	"net"
	tcp "tcp-simulator/internal"
)

func main() {
	ln, _ := net.Listen("tcp", ":8080")
	conn, _ := ln.Accept()
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var pkt tcp.Packet
	if err := dec.Decode(&pkt); err != nil {
		log.Println("decode error:", err)
		return
	}
	log.Printf("received packet: %+v", pkt)

	enc.Encode(tcp.Packet{Seq: 1, SYN: true, ACK: true})
	log.Println("sent SYN-ACK")

	if err := dec.Decode(&pkt); err != nil {
		log.Println("decode error:", err)
		return
	}
	log.Printf("received packet: %+v", pkt)
}
