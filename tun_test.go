package tun

import (
	"fmt"
	"net"
	"reflect"
	"sync/atomic"
	"testing"
	"time"
)

/*
ip tuntap add mode tun tun1
ifconfig tun1 inet 10.0.0.1 netmask 255.255.255.0 up
ifconfig tun1 inet6 add fd::1/120

go test -run ^TestAll$ tunat -count=1 -cover -v
*/
func TestAll(t *testing.T) {
	tun, udpRx, err := New(
		"tun1",
		&net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 100},
		&net.TCPAddr{IP: net.ParseIP("fd::1"), Port: 100},
		65535,
		100)
	if err != nil {
		panic(err)
	}

	testUDPWrite(tun, udpRx)
	testUDPRead(tun, udpRx)
	testTCP(tun, udpRx)
}

func testUDPWrite(tun *Tun, udpRx <-chan UDPData) {
	udpListener, _ := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("[::]"),
		Port: 100,
	})
	defer udpListener.Close()
	buf := make([]byte, 100)

	// ipv4
	_, err := tun.WriteTo(&net.UDPAddr{
		IP:   []byte{10, 0, 0, 2},
		Port: 100,
	}, []byte("abcd"), &net.UDPAddr{
		IP:   []byte{10, 0, 0, 1},
		Port: 100,
	})
	if err != nil {
		panic(err)
	}

	nread, _, _ := udpListener.ReadFrom(buf)
	if !reflect.DeepEqual(buf[:nread], []byte("abcd")) {
		panic("not equal")
	}

	// ipv6
	_, err = tun.WriteTo(&net.UDPAddr{
		IP:   net.ParseIP("fd::2"),
		Port: 100,
	}, []byte("abcd"), &net.UDPAddr{
		IP:   net.ParseIP("fd::1"),
		Port: 100,
	})
	if err != nil {
		panic(err)
	}

	nread, _, _ = udpListener.ReadFrom(buf)
	if !reflect.DeepEqual(buf[:nread], []byte("abcd")) {
		panic("not equal")
	}
}

func testUDPRead(tun *Tun, udpRx <-chan UDPData) {
	udpListener, _ := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("[::]"),
		Port: 100,
	})
	defer udpListener.Close()

	// ipv4
	_, err := udpListener.WriteTo([]byte("abcd"), &net.UDPAddr{
		IP:   net.ParseIP("10.0.0.2"),
		Port: 100,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(<-udpRx)

	// ipv6
	_, err = udpListener.WriteTo([]byte("abcd"), &net.UDPAddr{
		IP:   net.ParseIP("fd::2"),
		Port: 100,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(<-udpRx)
}

func testTCP(tun *Tun, udpRx <-chan UDPData) {
	tcpListener, _ := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP("[::]"), Port: 100})
	defer tcpListener.Close()

	// ipv4
	go func() {
		_, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP("10.0.0.2"), Port: 100})
		if err != nil {
			panic(err)
		}
	}()

	// ipv6
	go func() {
		_, err := net.DialTCP("tcp", nil, &net.TCPAddr{IP: net.ParseIP("fd::2"), Port: 100})
		if err != nil {
			panic(err)
		}
	}()

	for i := 0; i < 2; i++ {
		conn, _ := tcpListener.Accept()
		originSrcAddr, originDstAddr := tun.GetOriginSrcDst(conn.RemoteAddr().(*net.TCPAddr))
		fmt.Printf("%v -> %v   %v -> %v\n", conn.RemoteAddr(), conn.LocalAddr(), originSrcAddr, originDstAddr)

		// test map
		conn.Close()
		time.Sleep(10 * time.Millisecond)
		tun.DelNat(conn.RemoteAddr().(*net.TCPAddr))
	}

	// test map
	var flag uint32 = 0
	tun.tcpMap.Range(func(key, value interface{}) bool {
		atomic.StoreUint32(&flag, 1)
		fmt.Println(key.(string), value.(*tcpValue).natAddr, value.(*tcpValue).dstAddr)
		return true
	})
	if flag == 1 {
		panic("map has element")
	}
}
