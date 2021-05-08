// Package internal implements low-level BPF syscalls & interfaces.
// do NOT use this package. here be dragons.
// Use the higher level "packetfilter" interface instead....
// The internal package is based on, cilium's (MIT-licenced) eBPF library.
// I can't quite use their library because one of the kernel's I'm hoping to
// target (4.9-ish) doesn't have support for the BPF syscall that queries map information.
package internal

import (
	"fmt"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

type BPFCmd int

// Well known BPF commands.
const (
	BPF_MAP_CREATE BPFCmd = iota
	BPF_MAP_LOOKUP_ELEM
	BPF_MAP_UPDATE_ELEM
	BPF_MAP_DELETE_ELEM
	BPF_MAP_GET_NEXT_KEY
	BPF_PROG_LOAD
	BPF_OBJ_PIN
	BPF_OBJ_GET
)

const (
	BPF_ANY = iota
	BPF_NOEXIST
	BPF_EXIST
)

func BPF(cmd BPFCmd, attr unsafe.Pointer, size uintptr) (uintptr, error) {
	r1, _, errNo := unix.Syscall(unix.SYS_BPF, uintptr(cmd), uintptr(attr), size)
	runtime.KeepAlive(attr)
	var err error
	if errNo != 0 {
		err = errNo
	}

	return r1, err
}

type bpfObjAttr struct {
	fileName  Pointer
	fd        uint32
	fileFlags uint32
}

type bpfMapOpAttr struct {
	mapFd   uint32
	padding uint32
	key     Pointer
	value   Pointer
	flags   uint64
}

func BPFObjGet(fileName string, flags uint32) (*FD, error) {
	attr := bpfObjAttr{
		fileName:  NewStringPointer(fileName),
		fileFlags: flags,
	}
	ptr, err := BPF(BPF_OBJ_GET, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	if err != nil {
		return nil, fmt.Errorf("get object %s: %w", fileName, err)
	}
	return CreateFD(uint32(ptr)), nil
}

func BPFMapLookupElem(m *FD, key, valueOut Pointer) error {
	fd, err := m.Value()
	if err != nil {
		return err
	}

	attr := bpfMapOpAttr{
		mapFd: fd,
		key:   key,
		value: valueOut,
	}
	_, err = BPF(BPF_MAP_LOOKUP_ELEM, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	return err
}

func BPFMapUpdateElem(m *FD, key, valueOut Pointer, flags uint64) error {
	fd, err := m.Value()
	if err != nil {
		return err
	}

	attr := bpfMapOpAttr{
		mapFd: fd,
		key:   key,
		value: valueOut,
		flags: flags,
	}
	_, err = BPF(BPF_MAP_UPDATE_ELEM, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	return err
}

func BPFMapDeleteElem(m *FD, key Pointer) error {
	fd, err := m.Value()
	if err != nil {
		return err
	}

	attr := bpfMapOpAttr{
		mapFd: fd,
		key:   key,
	}
	_, err = BPF(BPF_MAP_DELETE_ELEM, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	return err
}

func BPFMapGetNextKey(m *FD, key, nextKeyOut Pointer) error {
	fd, err := m.Value()
	if err != nil {
		return err
	}

	attr := bpfMapOpAttr{
		mapFd: fd,
		key:   key,
		value: nextKeyOut,
	}
	_, err = BPF(BPF_MAP_GET_NEXT_KEY, unsafe.Pointer(&attr), unsafe.Sizeof(attr))
	return err
}
