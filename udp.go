package tunat

import (
	"net/netip"

	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
)

type udpData struct {
	payload []byte
	saddr   netip.AddrPort
	daddr   netip.AddrPort
}

// ReadFromUDPAddrPort like net package
func (t *Tunat) ReadFromUDPAddrPort(payload []byte) (nread int, saddr, daddr netip.AddrPort, err error) {
	udpData := <-t.udpChan
	nread = copy(payload, udpData.payload)
	return nread, udpData.saddr, udpData.daddr, nil
}

// WriteToUDPAddrPort like net package
func (t *Tunat) WriteToUDPAddrPort(payload []byte, saddr, daddr netip.AddrPort) (nwrite int, err error) {
	if saddr.Addr().Is4() {
		return t.ipv4WriteTo(payload, saddr, daddr)
	}
	return t.ipv6WriteTo(payload, saddr, daddr)
}

func (t *Tunat) ipv4WriteTo(payload []byte, saddr, daddr netip.AddrPort) (nwrite int, err error) {
	totalLen := header.IPv4MinimumSize + header.UDPMinimumSize + len(payload)
	ipHeader := header.IPv4(
		append(
			make([]byte, totalLen-len(payload)),
			payload...,
		),
	)
	ipHeader.Encode(&header.IPv4Fields{
		TotalLength: uint16(totalLen),
		TTL:         64,
		Protocol:    uint8(header.UDPProtocolNumber),
		SrcAddr:     tcpip.Address(saddr.Addr().AsSlice()),
		DstAddr:     tcpip.Address(daddr.Addr().AsSlice()),
	})
	ipHeader.SetChecksum(0)
	ipHeader.SetChecksum(^ipHeader.CalculateChecksum())

	udpHeader := header.UDP(ipHeader.Payload())
	udpHeader.Encode(&header.UDPFields{
		SrcPort: uint16(saddr.Port()),
		DstPort: uint16(daddr.Port()),
		Length:  uint16(header.UDPMinimumSize + len(payload)),
	})
	udpHeader.SetChecksum(0)
	udpHeader.SetChecksum(
		^udpHeader.CalculateChecksum(
			header.Checksum(
				udpHeader.Payload(),
				header.PseudoHeaderChecksum(
					header.UDPProtocolNumber,
					ipHeader.SourceAddress(),
					ipHeader.DestinationAddress(),
					udpHeader.Length(),
				),
			),
		),
	)

	return t.file.Write(ipHeader)
}

func (t *Tunat) ipv6WriteTo(payload []byte, saddr, daddr netip.AddrPort) (nwrite int, err error) {
	totalLen := header.IPv6MinimumSize + header.UDPMinimumSize + len(payload)
	ipHeader := header.IPv6(
		append(
			make([]byte, totalLen-len(payload)),
			payload...,
		),
	)
	ipHeader.Encode(&header.IPv6Fields{
		PayloadLength:     uint16(header.UDPMinimumSize + len(payload)),
		TransportProtocol: header.UDPProtocolNumber,
		HopLimit:          64,
		SrcAddr:           tcpip.Address(saddr.Addr().AsSlice()),
		DstAddr:           tcpip.Address(daddr.Addr().AsSlice()),
	})

	udpHeader := header.UDP(ipHeader.Payload())
	udpHeader.Encode(&header.UDPFields{
		SrcPort: uint16(saddr.Port()),
		DstPort: uint16(daddr.Port()),
		Length:  uint16(header.UDPMinimumSize + len(payload)),
	})
	udpHeader.SetChecksum(0)
	udpHeader.SetChecksum(
		^udpHeader.CalculateChecksum(
			header.Checksum(
				udpHeader.Payload(),
				header.PseudoHeaderChecksum(
					header.UDPProtocolNumber,
					ipHeader.SourceAddress(),
					ipHeader.DestinationAddress(),
					udpHeader.Length(),
				),
			),
		),
	)

	return t.file.Write(ipHeader)
}

func (t *Tunat) handleIPv4UDP(ipHeader header.IPv4, udpHeader header.UDP) {
	ip, ok := netip.AddrFromSlice([]byte(ipHeader.SourceAddress()))
	if !ok {
		return
	}
	saddr := netip.AddrPortFrom(ip, udpHeader.SourcePort())
	ip, ok = netip.AddrFromSlice([]byte(ipHeader.DestinationAddress()))
	if !ok {
		return
	}
	daddr := netip.AddrPortFrom(ip, udpHeader.DestinationPort())

	t.udpChan <- udpData{
		payload: append([]byte(nil), udpHeader.Payload()...),
		saddr:   saddr,
		daddr:   daddr,
	}
}

func (t *Tunat) handleIPv6UDP(ipHeader header.IPv6, udpHeader header.UDP) {
	ip, ok := netip.AddrFromSlice([]byte(ipHeader.SourceAddress()))
	if !ok {
		return
	}
	saddr := netip.AddrPortFrom(ip, udpHeader.SourcePort())
	ip, ok = netip.AddrFromSlice([]byte(ipHeader.DestinationAddress()))
	if !ok {
		return
	}
	daddr := netip.AddrPortFrom(ip, udpHeader.DestinationPort())

	t.udpChan <- udpData{
		payload: append([]byte(nil), udpHeader.Payload()...),
		saddr:   saddr,
		daddr:   daddr,
	}
}
