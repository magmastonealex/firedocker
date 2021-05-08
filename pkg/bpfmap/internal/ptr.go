package internal

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// NewPointer creates a 64-bit pointer from an unsafe Pointer.
func NewPointer(ptr unsafe.Pointer) Pointer {
	return Pointer{ptr: ptr}
}

// NewStringPointer creates a 64-bit pointer from a string.
func NewStringPointer(str string) Pointer {
	p, err := unix.BytePtrFromString(str)
	if err != nil {
		return Pointer{}
	}

	return Pointer{ptr: unsafe.Pointer(p)}
}
