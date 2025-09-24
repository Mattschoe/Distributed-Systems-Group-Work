package internal

import (
	"bytes"
	"encoding/binary"
)

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

func (tcp *TCP) ToBinary() []byte {
	var tcpBuffer bytes.Buffer
	binary.Write(&tcpBuffer, binary.BigEndian, tcp)
	return tcpBuffer.Bytes()
}

/*
 * Helper function to easily determine if package has a flag or not
 */
func (tcp *TCP) HasFlag(flag uint8) bool {
	return tcp.Flags&flag != 0
}

func ParseTCPSegment(data []byte) (*TCP, error) {
	var segment TCP
	buffer := bytes.NewReader(data)
	err := binary.Read(buffer, binary.BigEndian, &segment)
	if err != nil {
		return nil, err
	}
	return &segment, nil
}
