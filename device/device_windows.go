package device

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"golang.org/x/sys/windows"
	"golang.zx2c4.com/wintun"
)

type device struct {
	adapter       *wintun.Adapter
	session       wintun.Session
	readWaitEvent windows.Handle
	closeOnce     sync.Once
}

func (d *device) Read(buf []byte) (int, error) {
	for {
		packet, err := d.session.ReceivePacket()
		switch err {
		case nil:
			nread := copy(buf, packet)
			d.session.ReleaseReceivePacket(packet)
			return nread, err
		case windows.ERROR_NO_MORE_ITEMS:
			event, err := windows.WaitForSingleObject(d.readWaitEvent, windows.INFINITE)
			if err != nil {
				return 0, err
			}
			if event != windows.WAIT_OBJECT_0 {
				return 0, errors.New("event != windows.WAIT_OBJECT_0")
			}
		case windows.ERROR_HANDLE_EOF:
			return 0, os.ErrClosed
		case windows.ERROR_INVALID_DATA:
			return 0, errors.New("Send ring corrupt")
		default:
			return 0, fmt.Errorf("Read failed: %w", err)
		}
	}
}

func (d *device) Write(buf []byte) (int, error) {
	packet, err := d.session.AllocateSendPacket(len(buf))
	if err == nil {
		nwrite := copy(packet, buf)
		d.session.SendPacket(packet)
		return nwrite, nil
	}
	switch err {
	case windows.ERROR_HANDLE_EOF:
		return 0, os.ErrClosed
	case windows.ERROR_BUFFER_OVERFLOW:
		return 0, nil // Dropping when ring is full.
	}
	return 0, fmt.Errorf("Write failed: %w", err)
}

func (d *device) Close() (err error) {
	d.closeOnce.Do(func() {
		d.session.End()
		if d.adapter != nil {
			err = d.adapter.Close()
		}
	})
	return
}

// New create tun device
func New(name string) (file io.ReadWriteCloser, err error) {
	if _, err = os.Stat("wintun.dll"); err != nil {
		err = os.WriteFile("wintun.dll", wintunDLL, 0o777)
		if err != nil {
			return
		}
	}

	device := &device{}
	device.adapter, err = wintun.CreateAdapter(name, "WireGuard", nil)
	if err != nil {
		return nil, fmt.Errorf("Error creating interface: %w", err)
	}
	device.session, err = device.adapter.StartSession(0x800000)
	if err != nil {
		device.adapter.Close()
		return
	}
	device.readWaitEvent = device.session.ReadWaitEvent()
	return device, nil
}
