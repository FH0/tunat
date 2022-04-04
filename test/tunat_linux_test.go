package main

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"testing"
	"time"

	"github.com/FH0/tunat/device"
)

func TestUDP(t *testing.T) {
	testUDP()
}

func TestTCP(t *testing.T) {
	testTCP()
}

func TestUnixSocket(t *testing.T) {
	tn.Close()

	unixSocket := "/tmp/tunUnixSocket"

	// send fd background
	go func() {
		file, err := device.New("tun1")
		if err != nil {
			panic(err)
		}

		time.Sleep(10 * time.Millisecond)

		unixConn, err := net.DialUnix("unix", nil, &net.UnixAddr{
			Name: unixSocket,
			Net:  "unix",
		})
		if err != nil {
			panic(err)
		}

		oob := syscall.UnixRights(int(file.Fd()))
		_, _, err = unixConn.WriteMsgUnix([]byte("/dev/net/tun"), oob, nil)
		if err != nil {
			panic(err)
		}
	}()

	// recv fd
	file, err := device.NewFromUnixSocket(unixSocket)
	if err != nil {
		panic(err)
	}

	// send packet to tun background
	go func() {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("[::]"), Port: 100})
		if err != nil {
			panic(err)
		}

		_, err = conn.WriteToUDPAddrPort([]byte("abcd"), netip.MustParseAddrPort("[::FFFF:10.0.0.3]:100"))
		if err != nil {
			panic(err)
		}
	}()

	// read tun
	buf := make([]byte, 65535)
	nread, err := file.Read(buf)
	if err != nil {
		panic(err)
	}
	fmt.Println(hex.EncodeToString(buf[:nread]))
}
