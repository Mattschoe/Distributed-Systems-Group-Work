package main

type TCP struct {
	SourcePort        uint16
	DestinationPort   uint16
	Seq               uint32
	Ack               uint32
	OffsetAndReserved uint8 //Stored as: offset (4), reserved (4)
	Flags             uint8 //Stored as: CWR, ECE, URG, ACK, PSH, RST, SYN, FIN. Stored this way cuz accessing just 1 bit is illegal
	WindowSize        uint16
	Checksum          uint16
	Urgent            uint16
}

// Used to activate flags (For deactivating create new packets instead)
const (
	FIN_FLAG = 0x01
	SYN_FLAG = 0x02
	RST_FLAG = 0x04
	PSH_FLAG = 0x08
	ACK_FLAG = 0x10
	URG_FLAG = 0x20
	ECE_FLAG = 0x40
	CWR_FLAG = 0x80
)
