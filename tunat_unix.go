package tunat

import (
	"errors"
	"net"
	"net/netip"

	"github.com/FH0/tunat/device"
)

// NewFromUnixSocket new a Tunat from unix
func NewFromUnixSocket(path string, ipv4Prefix, ipv6Prefix netip.Prefix, bufLen int) (tunat *Tunat, err error) {
	tunat = &Tunat{
		udpChan: make(chan udpData, 100),
		bufLen:  bufLen,
	}

	tunat.file, err = device.NewFromUnixSocket(path)
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
