package tun

import (
	"os"
	"syscall"
	"unsafe"
)

type ifreq struct {
	ifrName  [syscall.IFNAMSIZ]byte
	ifrFlags uint16
}

// an issue https://github.com/golang/go/issues/30426#issuecomment-470335255
func tunAlloc(name string) (*os.File, error) {
	// open
	fd, err := syscall.Open("/dev/net/tun", os.O_RDWR|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}

	// ifreq set name
	var req ifreq
	copy(req.ifrName[:], []byte(name))
	req.ifrFlags = syscall.IFF_TUN | syscall.IFF_NO_PI

	// ioctl
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(fd),
		syscall.TUNSETIFF,
		uintptr(unsafe.Pointer(&req)),
	)
	if errno != 0 {
		return nil, errno
	}

	file := os.NewFile(uintptr(fd), "/dev/net/tun")
	return file, nil
}
