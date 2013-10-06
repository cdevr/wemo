package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

const (
	ssdpBroadcastGroup = "239.255.255.250:1900"
)

var intfName = flag.String("interface", "wlan0", "the interface to search for Wemo devices")
var on = flag.Bool("on", false, "turn the switches on ?")
var off = flag.Bool("off", false, "turn the switches off ?")

var MSEARCH = "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 10\r\nST: %s\r\nUSER-AGENT: unix/5.1 UPnP/1.1 crash/1.0\r\n\r\n"

func makeSSDPDiscoveryPacket(service string) string {
	return fmt.Sprintf(MSEARCH, service)
}

func getButtonState(location string) {
	if strings.HasPrefix(location, "http://") {
		location = location[7:]
	}
	location = strings.SplitN(location, "/", 2)[0]

	// Open a tcp connection.
	tcpConn, err := net.Dial("tcp", location)

	req := `<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:GetBinaryState xmlns:u="urn:Belkin:service:basicevent:1"></u:GetBinaryState>
  </s:Body>
</s:Envelope>

`

	header := fmt.Sprintf("POST http://%v/upnp/control/basicevent1 HTTP/1.1\r\nContent-type: text/xml; charset=\"utf-8\"\r\nSOAPACTION: \"urn:Belkin:service:basicevent:1#GetBinaryState\"\r\nContent-Length: %v\r\n\r\n", location, len(req))
	log.Printf("Sending : %v", header+req)
	tcpConn.Write([]byte(header + req))

	buf := make([]byte, 2048)
	n, err := tcpConn.Read(buf)

	if err != nil {
		log.Fatalf("Coulnd't read from %v", location)
	}
	response := string(buf[:n])
	log.Printf("Response : %v", response)

	idx := strings.Index(response, "<BinaryState>0</BinaryState>")
	if idx != -1 {
		log.Printf("Off !")
	}
	idx = strings.Index(response, "<BinaryState>1</BinaryState>")
	if idx != -1 {
		log.Printf("On !")
	}
	log.Printf("location found ! %v", location)
}

func changeButtonState(location string, state bool) {
	if strings.HasPrefix(location, "http://") {
		location = location[7:]
	}
	location = strings.SplitN(location, "/", 2)[0]

	// Open a tcp connection.
	tcpConn, err := net.Dial("tcp", location)

	toSet := 0
	if state {
		toSet = 1
	}

	req := fmt.Sprintf(`<?xml version="1.0" encoding="utf-8"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
  <s:Body>
    <u:SetBinaryState xmlns:u="urn:Belkin:service:basicevent:1">
      <BinaryState>%v</BinaryState>
    </u:SetBinaryState>
  </s:Body>
</s:Envelope>`, toSet)

	header := fmt.Sprintf("POST http://%v/upnp/control/basicevent1 HTTP/1.1\r\nContent-type: text/xml; charset=\"utf-8\"\r\nSOAPACTION: \"urn:Belkin:service:basicevent:1#SetBinaryState\"\r\nContent-Length: %v\r\n\r\n", location, len(req))
	log.Printf("Sending : %v", header+req)
	tcpConn.Write([]byte(header + req))

	buf := make([]byte, 2048)
	n, err := tcpConn.Read(buf)

	if err != nil {
		log.Fatalf("Coulnd't read from %v", location)
	}
	response := string(buf[:n])
	log.Printf("Response : %v", response)

	idx := strings.Index(response, "<BinaryState>0</BinaryState>")
	if idx != -1 {
		log.Printf("Off !")
	}
	idx = strings.Index(response, "<BinaryState>1</BinaryState>")
	if idx != -1 {
		log.Printf("On !")
	}
	log.Printf("location found ! %v", location)
}

func main() {
	flag.Parse()

	// Get all wemo devices.
	log.Printf("Trying to get all wemo devices on %v\n", *intfName)
	intf, err := net.InterfaceByName(*intfName)
	if err != nil {
		log.Fatalf("Couldn't get interface %v => %v", *intfName, err)
	}

	addrs, err := intf.Addrs()
	if err != nil {
		log.Fatalf("Coulnd't get interface addresses => %v", err)
	}
	laddr := ""
	for _, addr := range addrs {
		if strings.Index(addr.String(), ":") == -1 {
			laddr = strings.Split(addr.String(), "/")[0]
		}
		log.Printf("Found interface address %v\n", addr.String())
	}
	log.Printf("Using address %v", laddr)

	ludpAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:0", laddr))
	if err != nil {
		log.Fatalf("Couldn't resolve local udp address '%v' => %v", laddr, err)
	}

	// Make a UDP listening socket on an available port on this interface.
	udpConn, err := net.ListenUDP("udp", ludpAddr)
	defer udpConn.Close()
	if err != nil {
		log.Fatalf("Couldn't listen to a UDP port")
	}
	log.Printf("Listening to %v", udpConn.LocalAddr().String())

	maddr, err := net.ResolveUDPAddr("udp", ssdpBroadcastGroup)
	if err != nil {
		log.Fatalf("Couldn't resolve SSDP address on that interface\n")
	}

	log.Printf("Found maddr %v", maddr)

	// packet := makeSSDPDiscoveryPacket("ssdp:all") // "urn:Belkin:device:controllee:1")
	packet := makeSSDPDiscoveryPacket("urn:Belkin:device:controllee:1")

	log.Printf("Writing discovery packet")
	udpConn.WriteTo([]byte(packet), maddr)

	log.Printf("Setting read deadline")
	udpConn.SetReadDeadline(time.Now().Add(1 * time.Second))

	for {
		buffer := make([]byte, 2048)
		n, err := udpConn.Read(buffer)
		if err != nil {
			break
		}
		read := string(buffer[:n])
		lines := strings.Split(read, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "LOCATION: ") {
				if *on {
					go changeButtonState(line[10:], true)
				}
				if *off {
					go changeButtonState(line[10:], false)
				}
			}
		}
		log.Printf("Read : %v\n", string(buffer[:n]))
	}
}
