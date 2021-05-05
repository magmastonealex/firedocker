package main

import (
	"fmt"
	"os"
	"syscall"

	"golang.org/x/sys/unix"
)

type mountHelperImpl struct{}

// MustMkdir wraps os.MkdirAll, and panics if the call fails.
func (mh *mountHelperImpl) MustMkdir(path string, perm os.FileMode) {
	if err := os.MkdirAll(path, perm); err != nil {
		panic(err)
	}
}

// MustMount wraps syscall.Mount, and panics if the call fails.
func (mh *mountHelperImpl) MustMount(src string, dst string, typ string, flags uintptr, data string) {
	if err := syscall.Mount(src, dst, typ, flags, data); err != nil {
		panic(err)
	}
}

// MustSymlink wraps os.Symlink, and panics if the call fails.
func (mh *mountHelperImpl) MustSymlink(file string, newname string) {
	if err := os.Symlink(file, newname); err != nil {
		panic(err)
	}
}

// MustChdir wraps unix.Chdir, and panics if the call fails.
func (mh *mountHelperImpl) MustChdir(dir string) {
	if err := unix.Chdir(dir); err != nil {
		panic(err)
	}
}

// MustChroot wraps unix.Chroot, and panics if the call fails.
func (mh *mountHelperImpl) MustChroot(to string) {
	if err := unix.Chroot(to); err != nil {
		panic(fmt.Errorf("failed to chroot %v", err))
	}
}
