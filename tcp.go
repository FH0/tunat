package tun

import (
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func (t *Tun) handleTCP(networkLayer gopacket.NetworkLayer, tcp *layers.TCP) {
	if tcpPacket := t.handleTCPPacket(networkLayer, tcp); tcpPacket != nil {
		t.file.Write(tcpPacket)
	}
}

func (t *Tun) handleTCPPacket(networkLayer gopacket.NetworkLayer, tcp *layers.TCP) []byte {
	// modify
	var ipHeader gopacket.SerializableLayer
	switch _ipHeader := networkLayer.(type) {
	case *layers.IPv4:
		srcAddr := net.TCPAddr{
			IP:   _ipHeader.SrcIP,
			Port: int(tcp.SrcPort),
		}
		dstAddr := net.TCPAddr{
			IP:   _ipHeader.DstIP,
			Port: int(tcp.DstPort),
		}

		// syn
		if tcp.SYN && !tcp.ACK {
			if _, ok := t.tcpMap.Load(srcAddr.String()); !ok {
				i := 1
				for ; i < 65536; i++ {
					natAddr := net.TCPAddr{IP: t.fakeSrcAddr4, Port: i}
					if _, ok := t.tcpMap.Load(natAddr.String()); !ok {
						t.tcpMap.Store(natAddr.String(), &tcpValue{natAddr: srcAddr, dstAddr: &dstAddr})
						t.tcpMap.Store(srcAddr.String(), &tcpValue{natAddr: natAddr, dstAddr: &dstAddr})
						break
					}
				}
				if i == 65536 {
					return nil // FIXEME
				}
			}
		}

		/*
			tcpRedirect4 10.0.0.1:100
			raw        10.0.0.1:1234 -> 1.2.3.4:4321    SYN
			modified   10.0.0.2:1    -> 10.0.0.1:100    SYN
			raw        10.0.0.1:100  -> 10.0.0.2:1      ACK SYN
			modified   1.2.3.4:4321  -> 10.0.0.1:1234   ACK SYN
			raw        10.0.0.1:1234 -> 1.2.3.4:4321    ACK
			modified   10.0.0.2:1    -> 10.0.0.1:100    ACK
		*/
		if value, ok := t.tcpMap.Load(srcAddr.String()); ok {
			_ipHeader.SrcIP = value.(*tcpValue).natAddr.IP
			_ipHeader.DstIP = t.tcpRedirect4.IP
			tcp.SrcPort = layers.TCPPort(value.(*tcpValue).natAddr.Port)
			tcp.DstPort = layers.TCPPort(t.tcpRedirect4.Port)
		} else if value, ok := t.tcpMap.Load(dstAddr.String()); ok {
			_ipHeader.SrcIP = value.(*tcpValue).dstAddr.IP
			_ipHeader.DstIP = value.(*tcpValue).natAddr.IP
			tcp.SrcPort = layers.TCPPort(value.(*tcpValue).dstAddr.Port)
			tcp.DstPort = layers.TCPPort(value.(*tcpValue).natAddr.Port)
		} else {
			return nil
		}

		tcp.SetNetworkLayerForChecksum(_ipHeader)
		ipHeader = _ipHeader
	case *layers.IPv6:
		srcAddr := net.TCPAddr{
			IP:   _ipHeader.SrcIP,
			Port: int(tcp.SrcPort),
		}
		dstAddr := net.TCPAddr{
			IP:   _ipHeader.DstIP,
			Port: int(tcp.DstPort),
		}

		// syn
		if tcp.SYN && !tcp.ACK {
			if _, ok := t.tcpMap.Load(srcAddr.String()); !ok {
				i := 1
				for ; i < 65536; i++ {
					natAddr := net.TCPAddr{IP: t.fakeSrcAddr6, Port: i}
					if _, ok := t.tcpMap.Load(natAddr.String()); !ok {
						t.tcpMap.Store(natAddr.String(), &tcpValue{natAddr: srcAddr, dstAddr: &dstAddr})
						t.tcpMap.Store(srcAddr.String(), &tcpValue{natAddr: natAddr, dstAddr: &dstAddr})
						break
					}
				}
				if i == 65536 {
					return nil // FIXEME
				}
			}
		}

		if value, ok := t.tcpMap.Load(srcAddr.String()); ok {
			_ipHeader.SrcIP = value.(*tcpValue).natAddr.IP
			_ipHeader.DstIP = t.tcpRedirect6.IP
			tcp.SrcPort = layers.TCPPort(value.(*tcpValue).natAddr.Port)
			tcp.DstPort = layers.TCPPort(t.tcpRedirect6.Port)
		} else if value, ok := t.tcpMap.Load(dstAddr.String()); ok {
			_ipHeader.SrcIP = value.(*tcpValue).dstAddr.IP
			_ipHeader.DstIP = value.(*tcpValue).natAddr.IP
			tcp.SrcPort = layers.TCPPort(value.(*tcpValue).dstAddr.Port)
			tcp.DstPort = layers.TCPPort(value.(*tcpValue).natAddr.Port)
		} else {
			return nil
		}

		tcp.SetNetworkLayerForChecksum(_ipHeader)
		ipHeader = _ipHeader
	default:
		panic("TCP is either ipv4 or ipv6.")
	}

	// generate buf
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
	}
	gopacket.SerializeLayers(buf,
		opts,
		ipHeader,
		tcp,
		gopacket.Payload(tcp.Payload))

	return buf.Bytes()
}

type tcpValue struct {
	natAddr net.TCPAddr // fakeSrcAddr or originSrcAddr
	dstAddr *net.TCPAddr
}

func (t *Tun) GetOriginSrcDst(addr *net.TCPAddr) (*net.TCPAddr, *net.TCPAddr) {
	if value, ok := t.tcpMap.Load(addr.String()); ok {
		return &value.(*tcpValue).natAddr, value.(*tcpValue).dstAddr
	}

	return nil, nil
}

func (t *Tun) DelNat(addr *net.TCPAddr) {
	var natAddr *net.TCPAddr
	if value, ok := t.tcpMap.Load(addr.String()); ok {
		natAddr = &value.(*tcpValue).natAddr
	} else {
		return
	}

	t.tcpMap.Delete(natAddr.String())
	t.tcpMap.Delete(addr.String())
}
