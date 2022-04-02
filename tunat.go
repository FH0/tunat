package tunat

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/FH0/tunat/device"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// Tunat main struct
type Tunat struct {
	file                    io.ReadWriteCloser
	ipv4TCPListener         net.Listener
	ipv6TCPListener         net.Listener
	ipv4TCPListenerAddrPort netip.AddrPort
	ipv6TCPListenerAddrPort netip.AddrPort
	fakeIPv4Addr            netip.Addr
	fakeIPv6Addr            netip.Addr
	udpChan                 chan udpData
	bufLen                  int
	tcpMap                  sync.Map
}

// New new a Tunat
func New(name string, ipv4Prefix, ipv6Prefix netip.Prefix, bufLen int) (tunat *Tunat, err error) {
	tunat = &Tunat{
		udpChan: make(chan udpData, 100),
		bufLen:  bufLen,
	}

	tunat.file, err = device.New(name)
	if err != nil {
		return
	}
	if ipv4Prefix.IsValid() {
		tunat.ipv4TCPListener, err = net.Listen("tcp", netip.AddrPortFrom(ipv4Prefix.Addr(), 0).String())
		if err != nil {
			return
		}
		tunat.ipv4TCPListenerAddrPort = tunat.ipv4TCPListener.Addr().(*net.TCPAddr).AddrPort()
		tunat.fakeIPv4Addr = ipv4Prefix.Addr().Next()
		if !ipv4Prefix.Contains(tunat.fakeIPv4Addr) {
			err = errors.New("ipv4 next address is out of CIDR")
			return
		}
	}
	if ipv6Prefix.IsValid() {
		tunat.ipv6TCPListener, err = net.Listen("tcp", netip.AddrPortFrom(ipv6Prefix.Addr(), 0).String())
		if err != nil {
			return
		}
		tunat.ipv6TCPListenerAddrPort = tunat.ipv6TCPListener.Addr().(*net.TCPAddr).AddrPort()
		tunat.fakeIPv6Addr = ipv6Prefix.Addr().Next()
		if !ipv6Prefix.Contains(tunat.fakeIPv6Addr) {
			err = errors.New("ipv6 next address is out of CIDR")
			return
		}
	}

	go tunat.start()
	return
}

// Close close tun device
func (t *Tunat) Close() (err error) {
	return t.file.Close()
}

func (t *Tunat) start() {
	buf := make([]byte, t.bufLen)
	for {
		nread, err := t.file.Read(buf)
		if err != nil {
			fmt.Printf("tunat read error: %v\n", err)
			return
		}
		packet := buf[:nread]

		switch header.IPVersion(packet) {
		case header.IPv4Version:
			ipHeader := header.IPv4(packet)
			switch ipHeader.TransportProtocol() {
			case header.TCPProtocolNumber:
				t.handleIPv4TCP(ipHeader, ipHeader.Payload())
			case header.UDPProtocolNumber:
				t.handleIPv4UDP(ipHeader, ipHeader.Payload())
			}
		case header.IPv6Version:
			ipHeader := header.IPv6(packet)
			switch ipHeader.TransportProtocol() {
			case header.TCPProtocolNumber:
				t.handleIPv6TCP(ipHeader, ipHeader.Payload())
			case header.UDPProtocolNumber:
				t.handleIPv6UDP(ipHeader, ipHeader.Payload())
			}
		}
	}
}
