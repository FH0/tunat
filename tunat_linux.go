package tunat

import (
	"errors"
	"net"
	"net/netip"

	"github.com/FH0/tunat/device"
)

// NewFromUnixSocket new a Tunat from unix
func NewFromUnixSocket(path string,
	ipv4Prefix,
	ipv6Prefix netip.Prefix,
	bufLen int,
	preCommands []string,
	postCommands []string,
) (tunat *Tunat, err error) {
	tunat = &Tunat{
		udpChan: make(chan udpData, 100),
		bufLen:  bufLen,
	}

	err = excuteCommands(preCommands)
	if err != nil {
		return
	}
	tunat.file, err = device.NewFromUnixSocket(path)
	if err != nil {
		return
	}
	err = excuteCommands(postCommands)
	if err != nil {
		return
	}
	tunat.tcpListener, err = net.Listen("tcp", "[::]:0")
	if err != nil {
		return
	}
	if ipv4Prefix.IsValid() {
		tunat.ipv4TCPListenerAddrPort = netip.AddrPortFrom(
			ipv4Prefix.Addr(),
			uint16(tunat.tcpListener.Addr().(*net.TCPAddr).Port),
		)
		tunat.fakeIPv4Addr = ipv4Prefix.Addr().Next()
		if !ipv4Prefix.Contains(tunat.fakeIPv4Addr) {
			err = errors.New("ipv4 next address is out of CIDR")
			return
		}
	}
	if ipv6Prefix.IsValid() {
		tunat.ipv6TCPListenerAddrPort = netip.AddrPortFrom(
			ipv6Prefix.Addr(),
			uint16(tunat.tcpListener.Addr().(*net.TCPAddr).Port),
		)
		tunat.fakeIPv6Addr = ipv6Prefix.Addr().Next()
		if !ipv6Prefix.Contains(tunat.fakeIPv6Addr) {
			err = errors.New("ipv6 next address is out of CIDR")
			return
		}
	}

	go tunat.start()
	return
}
