package internal

import (
	"fmt"
	"runtime"

	"golang.org/x/sys/unix"
)

// FD type based roughly on cilium eBPF

type FD struct {
	raw int64
}

func CreateFD(value uint32) *FD {
	fd := &FD{int64(value)}
	runtime.SetFinalizer(fd, (*FD).Close)
	return fd
}

func (fd *FD) Value() (uint32, error) {
	if fd.raw < 0 {
		return 0, fmt.Errorf("fd is closed")
	}

	return uint32(fd.raw), nil
}

func (fd *FD) Close() error {
	if fd.raw < 0 {
		return nil
	}

	value := int(fd.raw)
	fd.raw = -1

	fd.Forget()
	return unix.Close(value)
}

func (fd *FD) Forget() {
	runtime.SetFinalizer(fd, nil)
}
