package tun

import (
	"errors"
	"net"
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

// Basically for Android
func readSocketFile(path string) (*os.File, error) {
	// listen
	unixListener, err := net.ListenUnix("unix", &net.UnixAddr{
		Name: path,
		Net:  "unix",
	})
	if err != nil {
		return nil, err
	}

	// accept
	unixConn, err := unixListener.AcceptUnix()
	if err != nil {
		return nil, err
	}

	// recvmsg
	name := make([]byte, 256)
	oob := make([]byte, syscall.CmsgSpace(4)) // fd length is 4
	nameLen, oobLen, _, _, err := unixConn.ReadMsgUnix(name, oob)
	if err != nil {
		return nil, err
	}

	// parse msg
	cmsgs, err := syscall.ParseSocketControlMessage(oob[:oobLen])
	if err != nil {
		return nil, err
	}
	if len(cmsgs) != 1 {
		return nil, errors.New("the number of cmsgs is not 1")
	}

	// get fd
	fds, err := syscall.ParseUnixRights(&cmsgs[0])
	if err != nil {
		return nil, err
	}
	if len(fds) != 1 {
		return nil, errors.New("the number of fds is not 1")
	}

	// unlink
	syscall.Unlink(path)

	file := os.NewFile(uintptr(fds[0]), string(name[:nameLen]))
	return file, nil
}
