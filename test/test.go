package main

import (
	"fmt"
	"net"
	"net/netip"
	"time"
)

type udpData struct {
	payload []byte
	saddr   netip.AddrPort
	daddr   netip.AddrPort
}

func testUDP() {
	udpListener, _ := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[::]:100")))
	defer udpListener.Close()
	buf := make([]byte, 100)

	// tunat write
	_, err := tn.WriteToUDPAddrPort(
		[]byte("abcd"),
		netip.MustParseAddrPort("10.0.0.3:100"),
		netip.MustParseAddrPort("10.0.0.1:100"),
	)
	if err != nil {
		panic(err)
	}
	_, err = tn.WriteToUDPAddrPort(
		[]byte("abcd"),
		netip.MustParseAddrPort("[fd::3]:100"),
		netip.MustParseAddrPort("[fd::1]:100"),
	)
	if err != nil {
		panic(err)
	}

	// udpListener write
	_, err = udpListener.WriteToUDPAddrPort([]byte("abcd"), netip.MustParseAddrPort("[::FFFF:10.0.0.3]:100"))
	if err != nil {
		panic(err)
	}
	_, err = udpListener.WriteToUDPAddrPort([]byte("abcd"), netip.MustParseAddrPort("[fd::3]:100"))
	if err != nil {
		panic(err)
	}

	// tunat read contains
	ch := make(chan udpData)
	go func() {
		for {
			nread, saddr, daddr, err := tn.ReadFromUDPAddrPort(buf)
			if err != nil {
				panic(err)
			}
			ch <- udpData{
				payload: append([]byte(nil), buf[:nread]...),
				saddr:   saddr,
				daddr:   daddr,
			}
		}
	}()
	var (
		tunatIPv4OK bool
		tunatIPv6OK bool
	)
	for !tunatIPv4OK || !tunatIPv6OK {
		select {
		case <-time.After(1 * time.Second):
			panic("timeout")
		case udpData := <-ch:
			if udpData.saddr.String() == "10.0.0.1:100" &&
				udpData.daddr.String() == "10.0.0.3:100" &&
				string(udpData.payload) == "abcd" {
				tunatIPv4OK = true
			}
			if udpData.saddr.String() == "[fd::1]:100" &&
				udpData.daddr.String() == "[fd::3]:100" &&
				string(udpData.payload) == "abcd" {
				tunatIPv6OK = true
			}
		}
	}

	// udpListener read
	nread, saddr, err := udpListener.ReadFromUDPAddrPort(buf)
	if err != nil {
		panic(err)
	}
	if (saddr.String() != "[::ffff:10.0.0.3]:100" && saddr.String() != "[fd::3]:100") ||
		string(buf[:nread]) != "abcd" {
		panic(fmt.Sprintf("%v %v", saddr, string(buf[:nread])))
	}
	nread, saddr, err = udpListener.ReadFromUDPAddrPort(buf)
	if err != nil {
		panic(err)
	}
	if (saddr.String() != "[::ffff:10.0.0.3]:100" && saddr.String() != "[fd::3]:100") ||
		string(buf[:nread]) != "abcd" {
		panic(fmt.Sprintf("%v %v", saddr, string(buf[:nread])))
	}
}

func testTCP() {
	buf := make([]byte, 100)

	// ipv4
	conn1, err := net.Dial("tcp", "10.0.0.3:100")
	if err != nil {
		panic(err)
	}
	conn2, err := tn.Accept()
	if err != nil {
		panic(err)
	}
	_, err = conn1.Write([]byte("abcd"))
	if err != nil {
		panic(err)
	}
	nread, err := conn2.Read(buf)
	if err != nil {
		panic(err)
	}
	if conn2.LocalAddr().String() != "10.0.0.3:100" || string(buf[:nread]) != "abcd" {
		panic(fmt.Sprintf("%v %v", conn1.LocalAddr(), string(buf[:nread])))
	}

	// ipv6
	conn1, err = net.Dial("tcp", "[fd::3]:100")
	if err != nil {
		panic(err)
	}
	conn2, err = tn.Accept()
	if err != nil {
		panic(err)
	}
	_, err = conn1.Write([]byte("abcd"))
	if err != nil {
		panic(err)
	}
	nread, err = conn2.Read(buf)
	if err != nil {
		panic(err)
	}
	if conn2.LocalAddr().String() != "[fd::3]:100" || string(buf[:nread]) != "abcd" {
		panic(fmt.Sprintf("%v %v", conn1.LocalAddr(), string(buf[:nread])))
	}
}
