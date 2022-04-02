package tunat

import (
	// "encoding/hex"
	// "fmt"
	"fmt"
	"net"
	"net/netip"

	// "sync/atomic"
	// "syscall"
	"testing"
	// "time"
)

var tunat *Tunat

/*
ip tuntap add mode tun tun1
ifconfig tun1 inet 10.0.0.1 netmask 255.255.255.0 up
ifconfig tun1 inet6 add fd::1/120

go test -count=1 -cover -v
*/
func init() {
	var err error
	tunat, err = New("tun1", netip.MustParsePrefix("10.0.0.1/24"), netip.MustParsePrefix("fd::1/120"), 1500)
	if err != nil {
		panic(err)
	}
}

func TestUDP(t *testing.T) {
	udpListener, _ := net.ListenUDP("udp", net.UDPAddrFromAddrPort(netip.MustParseAddrPort("[::]:100")))
	defer udpListener.Close()
	buf := make([]byte, 100)

	// ipv4
	_, err := udpListener.WriteToUDPAddrPort([]byte("abcd"), netip.MustParseAddrPort("[::FFFF:10.0.0.3]:100"))
	if err != nil {
		panic(err)
	}
	nread, saddr, daddr, err := tunat.ReadFromUDPAddrPort(buf)
	if err != nil {
		panic(err)
	}
	if saddr.String() != "10.0.0.1:100" ||
		daddr.String() != "10.0.0.3:100" ||
		string(buf[:nread]) != "abcd" {
		panic(fmt.Sprintf("%v %v %v\n", saddr, daddr, string(buf[:nread])))
	}

	_, err = tunat.WriteToUDPAddrPort(
		[]byte("abcd"),
		netip.MustParseAddrPort("10.0.0.3:100"),
		netip.MustParseAddrPort("10.0.0.1:100"),
	)
	if err != nil {
		panic(err)
	}
	nread, saddr, err = udpListener.ReadFromUDPAddrPort(buf)
	if err != nil {
		panic(err)
	}
	if saddr.String() != "[::ffff:10.0.0.3]:100" || string(buf[:nread]) != "abcd" {
		panic(fmt.Sprintf("%v %v\n", saddr, string(buf[:nread])))
	}

	// ipv6
	_, err = udpListener.WriteToUDPAddrPort([]byte("abcd"), netip.MustParseAddrPort("[fd::3]:100"))
	if err != nil {
		panic(err)
	}
	nread, saddr, daddr, err = tunat.ReadFromUDPAddrPort(buf)
	if err != nil {
		panic(err)
	}
	if saddr.String() != "[fd::1]:100" ||
		daddr.String() != "[fd::3]:100" ||
		string(buf[:nread]) != "abcd" {
		panic(fmt.Sprintf("%v %v %v\n", saddr, daddr, string(buf[:nread])))
	}

	_, err = tunat.WriteToUDPAddrPort(
		[]byte("abcd"),
		netip.MustParseAddrPort("[fd::3]:100"),
		netip.MustParseAddrPort("[fd::1]:100"),
	)
	if err != nil {
		panic(err)
	}
	nread, saddr, err = udpListener.ReadFromUDPAddrPort(buf)
	if err != nil {
		panic(err)
	}
	if saddr.String() != "[fd::3]:100" || string(buf[:nread]) != "abcd" {
		panic(fmt.Sprintf("%v %v\n", saddr, string(buf[:nread])))
	}
}

func TestTCP(t *testing.T) {
	buf := make([]byte, 100)

	// ipv4
	conn1, err := net.Dial("tcp", "10.0.0.3:100")
	if err != nil {
		panic(err)
	}
	conn2, err := tunat.IPv4Accept()
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
		panic(fmt.Sprintf("%v %v\n", conn1.LocalAddr(), string(buf[:nread])))
	}

	// ipv6
	conn1, err = net.Dial("tcp", "[fd::3]:100")
	if err != nil {
		panic(err)
	}
	conn2, err = tunat.IPv6Accept()
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
		panic(fmt.Sprintf("%v %v\n", conn1.LocalAddr(), string(buf[:nread])))
	}
}

// func TestSocketFile(t *testing.T) {
// 	tunat.file.Close()

// 	socketFile := "/tmp/tunSocket"

// 	// send fd background
// 	go func() {
// 		file, err := tunAlloc("tun1")
// 		if err != nil {
// 			panic(err)
// 		}

// 		time.Sleep(10 * time.Millisecond)

// 		unixConn, err := net.DialUnix("unix", nil, &net.UnixAddr{
// 			Name: socketFile,
// 			Net:  "unix",
// 		})
// 		if err != nil {
// 			panic(err)
// 		}

// 		oob := syscall.UnixRights(int(file.Fd()))
// 		_, _, err = unixConn.WriteMsgUnix([]byte("/dev/net/tun"), oob, nil)
// 		if err != nil {
// 			panic(err)
// 		}
// 	}()

// 	// recv fd
// 	file, err := readSocketFile(socketFile)
// 	if err != nil {
// 		panic(err)
// 	}

// 	// send packet to tun background
// 	go func() {
// 		conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: 100})
// 		if err != nil {
// 			panic(err)
// 		}

// 		_, err = conn.WriteTo([]byte("abcd"), &net.UDPAddr{IP: net.ParseIP("[::FFFF:10.0.0.3]"), Port: 100})
// 		if err != nil {
// 			panic(err)
// 		}
// 	}()

// 	// read tun
// 	buf := make([]byte, 65535)
// 	nread, err := file.Read(buf)
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Println(hex.EncodeToString(buf[:nread]))
// }
