package tunat

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/netip"
	"os/exec"
	"strings"
	"sync"

	"github.com/FH0/tunat/device"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

// Tunat main struct
type Tunat struct {
	file                    io.ReadWriteCloser
	tcpListener             net.Listener
	ipv4TCPListenerAddrPort netip.AddrPort
	ipv6TCPListenerAddrPort netip.AddrPort
	fakeIPv4Addr            netip.Addr
	fakeIPv6Addr            netip.Addr
	udpChan                 chan udpData
	bufLen                  int
	tcpMap                  sync.Map
}

// New new a Tunat
func New(name string,
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
	tunat.file, err = device.New(name)
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

func excuteCommands(commands []string) (err error) {
	for _, cmd := range commands {
		cmdSlice := strings.Split(cmd, " ")
		var out []byte
		switch len(cmdSlice) {
		case 0:
			return errors.New("invalid command")
		case 1:
			out, err = exec.Command(cmdSlice[0]).CombinedOutput()
		default:
			out, err = exec.Command(cmdSlice[0], cmdSlice[1:]...).CombinedOutput()
		}
		if err != nil {
			err = fmt.Errorf("%v: %v", string(out), err.Error())
			return
		}
	}
	return
}
