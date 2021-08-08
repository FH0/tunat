package tun

import (
	"net"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func (t *Tun) WriteTo(srcAddr *net.UDPAddr, payload []byte, dstAddr *net.UDPAddr) (int, error) {
	// udpHeader
	udpHeader := &layers.UDP{
		SrcPort: layers.UDPPort(srcAddr.Port),
		DstPort: layers.UDPPort(dstAddr.Port),
	}

	// ipHeader
	var ipHeader gopacket.SerializableLayer
	if len(srcAddr.IP) == 4 {
		_ipHeader := layers.IPv4{
			BaseLayer: layers.BaseLayer{},
			Version:   4,
			TTL:       64,
			Protocol:  layers.IPProtocolUDP,
			SrcIP:     srcAddr.IP,
			DstIP:     dstAddr.IP,
		}
		udpHeader.SetNetworkLayerForChecksum(&_ipHeader)
		ipHeader = &_ipHeader
	} else {
		_ipHeader := layers.IPv6{
			Version:    6,
			NextHeader: layers.IPProtocolUDP,
			HopLimit:   64,
			SrcIP:      srcAddr.IP,
			DstIP:      dstAddr.IP,
		}
		udpHeader.SetNetworkLayerForChecksum(&_ipHeader)
		ipHeader = &_ipHeader
	}

	// generate buf
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		FixLengths:       true,
		ComputeChecksums: true,
	}
	if err := gopacket.SerializeLayers(buf, opts,
		ipHeader,
		udpHeader,
		gopacket.Payload(payload)); err != nil {
		return 0, err
	}

	// write
	return t.file.Write(buf.Bytes())
}

func (t *Tun) handleUDP(networkLayer gopacket.NetworkLayer, udpLayer *layers.UDP) {
	t.udpTx <- UDPData{
		SrcAddr: &net.UDPAddr{
			IP:   networkLayer.NetworkFlow().Src().Raw(),
			Port: int(udpLayer.SrcPort),
			Zone: "",
		},
		Data: udpLayer.Payload,
		DstAddr: &net.UDPAddr{
			IP:   networkLayer.NetworkFlow().Dst().Raw(),
			Port: int(udpLayer.DstPort),
			Zone: "",
		},
	}
}

type UDPData struct {
	SrcAddr net.Addr
	Data    []byte
	DstAddr net.Addr
}
