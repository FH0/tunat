package tunat

import (
	"encoding/hex"
	"fmt"
	"net"
	"reflect"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

var tunat Tunat

/*
ip tuntap add mode tun tun1
ifconfig tun1 inet 10.0.0.1 netmask 255.255.255.0 up
ifconfig tun1 inet6 add fd::1/120

go test -count=1 -cover -v
*/
func init() {
	var err error
	tunat, err = New(
		"tun1",
		"",
		&net.TCPAddr{IP: net.ParseIP("10.0.0.1"), Port: 100},
		&net.TCPAddr{IP: net.ParseIP("fd::1"), Port: 100},
		65535,
		100)
	if err != nil {
		panic(err)
	}
}

func TestUDPWrite(t *testing.T) {
	udpListener, _ := net.ListenUDP("udp", &net.UDPAddr{
		IP:   net.ParseIP("[::]"),
		Port: 100,
	})
	defer udpListener.Close()
	buf := make([]byte, 100)

	// ipv4
	_, err := tunat.WriteTo(&net.UDPAddr{
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
	_, err = tunat.WriteTo(&net.UDPAddr{
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

func TestUDPRead(t *testing.T) {
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
	fmt.Println(<-tunat.UDPRx)

	// ipv6
	_, err = udpListener.WriteTo([]byte("abcd"), &net.UDPAddr{
		IP:   net.ParseIP("fd::2"),
		Port: 100,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(<-tunat.UDPRx)
}

func TestTCP(t *testing.T) {
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
		originSrcAddr, originDstAddr := tunat.GetSrcDst(conn.RemoteAddr().(*net.TCPAddr))
		fmt.Printf("%v -> %v   %v -> %v\n", conn.RemoteAddr(), conn.LocalAddr(), originSrcAddr, originDstAddr)

		// test map
		conn.Close()
		time.Sleep(10 * time.Millisecond)
		tunat.DelNat(conn.RemoteAddr().(*net.TCPAddr))
	}

	// test map
	var flag uint32
	tunat.tcpMap.Range(func(key, value interface{}) bool {
		atomic.StoreUint32(&flag, 1)
		fmt.Println(key.(string), value.(*tcpValue).natAddr, value.(*tcpValue).dstAddr)
		return true
	})
	if flag == 1 {
		panic("map has element")
	}
}

func TestSocketFile(t *testing.T) {
	tunat.file.Close()

	socketFile := "/tmp/tunSocket"

	// send fd background
	go func() {
		file, err := tunAlloc("tun1")
		if err != nil {
			panic(err)
		}

		time.Sleep(10 * time.Millisecond)

		unixConn, err := net.DialUnix("unix", nil, &net.UnixAddr{
			Name: socketFile,
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
	file, err := readSocketFile(socketFile)
	if err != nil {
		panic(err)
	}

	// send packet to tun background
	go func() {
		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 100})
		if err != nil {
			panic(err)
		}

		_, err = conn.WriteTo([]byte("abcd"), &net.UDPAddr{IP: net.ParseIP("10.0.0.2"), Port: 100})
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
