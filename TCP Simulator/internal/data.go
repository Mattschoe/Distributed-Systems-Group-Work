package internal

type Request struct {
	SYN chan struct{}
	ACK chan struct{}
}
