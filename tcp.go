package tunat

import (
	"errors"
	"net"
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type tcpMapValue struct {
	natAddr netip.AddrPort // fakeSAddr or originSAddr
	daddr   netip.AddrPort
}

type tcpConn struct {
	net.Conn
	tunat          *Tunat
	saddr          netip.AddrPort
	daddr          netip.AddrPort
	saddrInterface net.Addr
	daddrInterface net.Addr
}

// Close delete nat map at the same time
func (tc *tcpConn) Close() error {
	if value, ok := tc.tunat.tcpMap.Load(tc.saddr); ok {
		tc.tunat.tcpMap.Delete(value.(*tcpMapValue).natAddr)
		tc.tunat.tcpMap.Delete(tc.saddr)
	}

	return tc.Conn.Close()
}

// LocalAddr original destination address
func (tc *tcpConn) LocalAddr() net.Addr {
	return tc.daddrInterface
}

// RemoteAddr original source address
func (tc *tcpConn) RemoteAddr() net.Addr {
	return tc.saddrInterface
}

// Accept like net package
func (t *Tunat) Accept() (conn net.Conn, err error) {
	acceptConn, err := t.tcpListener.Accept()
	if err != nil {
		return
	}

	connRemoteAddr := acceptConn.RemoteAddr().(*net.TCPAddr).AddrPort()
	if connRemoteAddr.Addr().Is4In6() {
		connRemoteAddr = netip.AddrPortFrom(connRemoteAddr.Addr().Unmap(), connRemoteAddr.Port())
	}
	value, ok := t.tcpMap.Load(connRemoteAddr)
	if !ok {
		return nil, errors.New("tcp nat map not exist")
	}
	saddr := value.(*tcpMapValue).natAddr
	daddr := value.(*tcpMapValue).daddr
	conn = &tcpConn{
		Conn:           acceptConn,
		tunat:          t,
		saddr:          saddr,
		daddr:          daddr,
		saddrInterface: net.TCPAddrFromAddrPort(saddr),
		daddrInterface: net.TCPAddrFromAddrPort(daddr),
	}
	return
}

func (t *Tunat) handleIPv4TCP(ipHeader header.IPv4, tcpHeader header.TCP) {
	/*
		tcpListener	10.0.0.1:100
		raw			10.0.0.1:1234	->	1.2.3.4:4321	SYN
		modified	10.0.0.2:1234	->	10.0.0.1:100	SYN
		raw			10.0.0.1:100	->	10.0.0.2:1234	ACK SYN
		modified	1.2.3.4:4321	->	10.0.0.1:1234	ACK SYN
		raw			10.0.0.1:1234	->	1.2.3.4:4321	ACK
		modified	10.0.0.2:1234	->	10.0.0.1:100	ACK
	*/
	ip, ok := netip.AddrFromSlice([]byte(ipHeader.SourceAddress()))
	if !ok {
		return
	}
	saddr := netip.AddrPortFrom(ip, tcpHeader.SourcePort())
	ip, ok = netip.AddrFromSlice([]byte(ipHeader.DestinationAddress()))
	if !ok {
		return
	}
	daddr := netip.AddrPortFrom(ip, tcpHeader.DestinationPort())

	if tcpHeader.Flags()&header.TCPFlagSyn == 0 || tcpHeader.Flags()&header.TCPFlagAck != 0 {
		goto next
	}
	if _, ok := t.tcpMap.Load(saddr); ok {
		goto next
	}
	for port, endPort := tcpHeader.SourcePort(), tcpHeader.SourcePort()-1; port != endPort; port++ {
		if port == 0 {
			continue
		}
		fakeAddr := netip.AddrPortFrom(t.fakeIPv4Addr, port)
		if _, ok := t.tcpMap.Load(fakeAddr); !ok {
			t.tcpMap.Store(fakeAddr, &tcpMapValue{natAddr: saddr, daddr: daddr})
			t.tcpMap.Store(saddr, &tcpMapValue{natAddr: fakeAddr, daddr: daddr})
			goto next
		}
	}
	return

next:
	if value, ok := t.tcpMap.Load(saddr); ok {
		ipHeader.SetSourceAddress(tcpip.Address(value.(*tcpMapValue).natAddr.Addr().AsSlice()))
		ipHeader.SetDestinationAddress(tcpip.Address(t.ipv4TCPListenerAddrPort.Addr().AsSlice()))
		tcpHeader.SetSourcePort(uint16(value.(*tcpMapValue).natAddr.Port()))
		tcpHeader.SetDestinationPort(uint16(t.ipv4TCPListenerAddrPort.Port()))
	} else if value, ok := t.tcpMap.Load(daddr); ok {
		ipHeader.SetSourceAddress(tcpip.Address(value.(*tcpMapValue).daddr.Addr().AsSlice()))
		ipHeader.SetDestinationAddress(tcpip.Address(value.(*tcpMapValue).natAddr.Addr().AsSlice()))
		tcpHeader.SetSourcePort(uint16(value.(*tcpMapValue).daddr.Port()))
		tcpHeader.SetDestinationPort(uint16(value.(*tcpMapValue).natAddr.Port()))
	} else {
		return
	}

	ipHeader.SetChecksum(0)
	ipHeader.SetChecksum(^ipHeader.CalculateChecksum())
	tcpHeader.SetChecksum(0)
	tcpHeader.SetChecksum(
		^tcpHeader.CalculateChecksum(
			header.Checksum(
				tcpHeader.Payload(),
				header.PseudoHeaderChecksum(
					header.TCPProtocolNumber,
					ipHeader.SourceAddress(),
					ipHeader.DestinationAddress(),
					uint16(len(tcpHeader)),
				),
			),
		),
	)

	_, _ = t.file.Write(ipHeader)
}

func (t *Tunat) handleIPv6TCP(ipHeader header.IPv6, tcpHeader header.TCP) {
	ip, ok := netip.AddrFromSlice([]byte(ipHeader.SourceAddress()))
	if !ok {
		return
	}
	saddr := netip.AddrPortFrom(ip, tcpHeader.SourcePort())
	ip, ok = netip.AddrFromSlice([]byte(ipHeader.DestinationAddress()))
	if !ok {
		return
	}
	daddr := netip.AddrPortFrom(ip, tcpHeader.DestinationPort())

	if tcpHeader.Flags()&header.TCPFlagSyn == 0 || tcpHeader.Flags()&header.TCPFlagAck != 0 {
		goto next
	}
	if _, ok := t.tcpMap.Load(saddr); ok {
		goto next
	}
	for port, endPort := tcpHeader.SourcePort(), tcpHeader.SourcePort()-1; port != endPort; port++ {
		if port == 0 {
			continue
		}
		fakeAddr := netip.AddrPortFrom(t.fakeIPv6Addr, port)
		if _, ok := t.tcpMap.Load(fakeAddr); !ok {
			t.tcpMap.Store(fakeAddr, &tcpMapValue{natAddr: saddr, daddr: daddr})
			t.tcpMap.Store(saddr, &tcpMapValue{natAddr: fakeAddr, daddr: daddr})
			goto next
		}
	}
	return

next:
	if value, ok := t.tcpMap.Load(saddr); ok {
		ipHeader.SetSourceAddress(tcpip.Address(value.(*tcpMapValue).natAddr.Addr().AsSlice()))
		ipHeader.SetDestinationAddress(tcpip.Address(t.ipv6TCPListenerAddrPort.Addr().AsSlice()))
		tcpHeader.SetSourcePort(uint16(value.(*tcpMapValue).natAddr.Port()))
		tcpHeader.SetDestinationPort(uint16(t.ipv6TCPListenerAddrPort.Port()))
	} else if value, ok := t.tcpMap.Load(daddr); ok {
		ipHeader.SetSourceAddress(tcpip.Address(value.(*tcpMapValue).daddr.Addr().AsSlice()))
		ipHeader.SetDestinationAddress(tcpip.Address(value.(*tcpMapValue).natAddr.Addr().AsSlice()))
		tcpHeader.SetSourcePort(uint16(value.(*tcpMapValue).daddr.Port()))
		tcpHeader.SetDestinationPort(uint16(value.(*tcpMapValue).natAddr.Port()))
	} else {
		return
	}

	tcpHeader.SetChecksum(0)
	tcpHeader.SetChecksum(
		^tcpHeader.CalculateChecksum(
			header.Checksum(
				tcpHeader.Payload(),
				header.PseudoHeaderChecksum(
					header.TCPProtocolNumber,
					ipHeader.SourceAddress(),
					ipHeader.DestinationAddress(),
					uint16(len(tcpHeader)),
				),
			),
		),
	)

	_, _ = t.file.Write(ipHeader)
}
