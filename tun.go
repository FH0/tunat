package tun

import (
	"fmt"
	"net"
	"os"
	"sync"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

type Tun struct {
	file         *os.File
	tcpRedirect4 *net.TCPAddr
	tcpRedirect6 *net.TCPAddr
	fakeSrcAddr4 net.IP
	fakeSrcAddr6 net.IP
	bufLen       int
	tcpMap       sync.Map
	udpTx        chan<- UDPData
}

func New(
	name string,
	socketFile string,
	tcpRedirect4 *net.TCPAddr, // can't be the end of network because fakeSrcAddr is the next addr
	tcpRedirect6 *net.TCPAddr, // can't be the end of network because fakeSrcAddr is the next addr
	bufLen int,
	udpChanCapacity int,
) (*Tun, <-chan UDPData, error) {
	// file
	var (
		file *os.File
		err  error
	)
	if name != "" {
		file, err = tunAlloc(name)
		if err != nil {
			return nil, nil, err
		}
	} else {
		file, err = readSocketFile(socketFile)
		if err != nil {
			return nil, nil, err
		}
	}

	// fakeSrcAddr
	var fakeSrcAddr4 net.IP
	if tcpRedirect4 != nil {
		fakeSrcAddr4 = append([]byte(nil), tcpRedirect4.IP...)
		fakeSrcAddr4[len(tcpRedirect4.IP)-1] += 1
	}
	var fakeSrcAddr6 net.IP
	if tcpRedirect6 != nil {
		fakeSrcAddr6 = append([]byte(nil), tcpRedirect6.IP...)
		fakeSrcAddr6[len(tcpRedirect6.IP)-1] += 1
	}

	// struct
	udpChan := make(chan UDPData, udpChanCapacity)
	tun := &Tun{
		file:         file,
		tcpRedirect4: tcpRedirect4,
		tcpRedirect6: tcpRedirect6,
		fakeSrcAddr4: fakeSrcAddr4,
		fakeSrcAddr6: fakeSrcAddr6,
		bufLen:       bufLen,
		udpTx:        udpChan,
	}

	// background
	go tun.start()

	return tun, udpChan, nil
}

func (t *Tun) start() {
	for {
		buf := make([]byte, t.bufLen) // in loop because udpChan need
		nread, err := t.file.Read(buf)
		if err != nil {
			fmt.Printf("tunat read error: %v\n", err)
			return
		}

		// at least 20
		if nread <= 20 {
			continue
		}

		// decoder
		var decoder gopacket.Decoder
		switch buf[0] >> 4 {
		case 4:
			decoder = layers.LayerTypeIPv4
		case 6:
			decoder = layers.LayerTypeIPv6
		default:
			continue
		}

		// decode
		packet := gopacket.NewPacket(buf[:nread], decoder, gopacket.DecodeOptions{
			Lazy:   true,
			NoCopy: true,
		})

		// protocol
		if tcpLayer := packet.Layer(layers.LayerTypeTCP); tcpLayer != nil {
			t.handleTCP(packet.NetworkLayer(), tcpLayer.(*layers.TCP))
		} else if udpLayer := packet.Layer(layers.LayerTypeUDP); udpLayer != nil {
			t.handleUDP(packet.NetworkLayer(), udpLayer.(*layers.UDP))
		}
	}
}
