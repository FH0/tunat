package device

import (
	"errors"
	"io"
	"net"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

type ifreq struct {
	ifrName  [syscall.IFNAMSIZ]byte
	ifrFlags uint16
}

// New an issue https://github.com/golang/go/issues/30426#issuecomment-470335255
func New(name string) (file io.ReadWriteCloser, err error) {
	var tunPath string
	if _, err = os.Stat("/dev/net/tun"); err == nil {
		tunPath = "/dev/net/tun"
	} else if _, err = os.Stat("/dev/tun"); err == nil {
		tunPath = "/dev/tun"
	} else {
		return
	}

	fd, err := syscall.Open(tunPath, os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return
	}

	var req ifreq
	copy(req.ifrName[:], []byte(name))
	req.ifrFlags = syscall.IFF_TUN | syscall.IFF_NO_PI

	_, _, errno := unix.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		syscall.TUNSETIFF,
		uintptr(unsafe.Pointer(&req)),
	)
	if errno != 0 {
		return nil, errno
	}

	file = os.NewFile(uintptr(fd), tunPath)
	return
}

// NewFromUnixSocket basically for Android
func NewFromUnixSocket(path string) (device *os.File, err error) {
	unixListener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: path,
		Net:  "unix",
	})
	if err != nil {
		return
	}

	unixConn, err := unixListener.AcceptUnix()
	if err != nil {
		return
	}

	name := make([]byte, 256)
	oob := make([]byte, syscall.CmsgSpace(4)) // fd length is 4
	nameLen, oobLen, _, _, err := unixConn.ReadMsgUnix(name, oob)
	if err != nil {
		return
	}

	// parse msg
	cmsgs, err := syscall.ParseSocketControlMessage(oob[:oobLen])
	if err != nil {
		return
	}
	if len(cmsgs) != 1 {
		return nil, errors.New("the number of cmsgs is not 1")
	}

	// get fd from msg
	fds, err := syscall.ParseUnixRights(&cmsgs[0])
	if err != nil {
		return
	}
	if len(fds) != 1 {
		return nil, errors.New("the number of fds is not 1")
	}

	err = syscall.Unlink(path)
	if err != nil {
		return
	}

	file := os.NewFile(uintptr(fds[0]), string(name[:nameLen]))
	return file, nil
}
