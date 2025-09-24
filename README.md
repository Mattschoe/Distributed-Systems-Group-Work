# TCP/IP Simulator using the net package
The solution uses the net package in GoLang to simulate the TCP handshake during 
connection between a client and server. The solution also simulates sending messages
and shows how the server/client interacts when some of the messages are either out of order,
corrupted or/and received wrong. To best simulate the process, the solution uses _udp_ as
it's protocol for the network layer, instead of using the standard _tcp_, which is
also available in the net package.

#### a) Data structure for transmitting data and meta-data
To transmit meta data the solution uses a mock TCP header struct, which is defined 
like so:
```go
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
```
The struct is 20 bytes of size and is used to establish a server/client connection
with the 3-way handshake. For sending the messages between client and server, the 
solution uses simple integers to demonstrate the process. The actual protocol 
also includes the tcp header when sending messages, but this was omitted from 
this demonstration. 

#### b) Threads or Processes in Solution
The solution uses processes simulate server/client interation, instead of threads. 
This is because it's unrealistic to use threads since processes are independent of 
eachother, whereas threads are a subset of a process and therefore not independent. 
Threads also share the same memory[[1]](https://bytebytego.com/guides/what-is-the-difference-between-process-and-thread/), 
which wouldn't make sense for a server and client.

#### c) Handling out-of-order messages
When receiving a segment of messages from the client the user collects the segment in 
an array. This array is ordered in the order of messages and **not** ordered in when
messages are sent. So if the client sends messages out-of-order the solution fixes this
by just putting the message in their index position in the array, which then is ordered
and also complete eafter receiving the whole segment. If the segment is NOT complete (some
index position _i_ has the default value 0 and not the message value) the server fixes this 
by requesting the missing/corrupted package from the client when sending acknowledgements. 

#### d) Handling message loss
When the server returns acknowledgement for the messages it checks if the messages are 
what the server expects to receive (for this demonstration it does so simply by just
checking if the message value matches the index position of the output array). If they
are not what the server expects the server sends a request to the client for the 
missing/corrupted package, and then waits for the client to send the correct package.
After the exchange the server continues sending acknowledgements for the other messages.
In a better solution, the server wouldn't wait for a clients response, but simply send
acknowledgements for the other messages while waiting. 

#### e) The importance of 3-way handshake
The 3-way handshake is important because it helps TCP establish a reliable connection
before data transmission beings. A lot can go wrong on client/server connection, but the
3-way handshake helps ensure that both parties are ready for communication[[2]](https://www.geeksforgeeks.org/computer-networks/tcp-3-way-handshake-process/).
