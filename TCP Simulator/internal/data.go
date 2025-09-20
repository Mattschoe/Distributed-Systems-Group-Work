package internal

type Packet struct {
	Seq int
	SYN bool
	ACK bool
}
