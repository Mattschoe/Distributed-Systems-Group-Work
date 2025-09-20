package main

import (
	"encoding/json"
	"log"
	"net"
	tcp "tcp-simulator/internal"
)

func main() {
	conn, _ := net.Dial("tcp", "localhost:8080")
	dec := json.NewDecoder(conn)
	enc := json.NewEncoder(conn)

	var pkt tcp.Packet

	enc.Encode(tcp.Packet{Seq: 1, SYN: true})
	log.Println("sent SYN")

	if err := dec.Decode(&pkt); err != nil {
		log.Println("decode error:", err)
		return
	}
	log.Printf("received packet: %+v", pkt)

	enc.Encode(tcp.Packet{Seq: 2, ACK: true})
	log.Println("sent ACK")

}
