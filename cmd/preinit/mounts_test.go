package main

//go:generate mockery --name=mountHelper --structname=MountHelperMock

import (
	"firedocker/cmd/preinit/mocks"
	"os"
	"syscall"
	"testing"
)

// This looks like a reverse implementation... but it's not quite.
// It's based on system-manager docs, as well as a few resources on having an overlay rootfs.
// The intention of this test is to prove everything that needs calling is being called.
// When the recursive deletion of the initramfs is done, then this will likely be a more useful test.
// For now it's here to prove my refactors didn't break anything
func TestMountsProperly(t *testing.T) {
	mh := new(mocks.MountHelperMock)
	mh.On("MustMkdir", "/ro", os.FileMode(0755))
	mh.On("MustMkdir", "/rw", os.FileMode(0755))
	mh.On("MustMkdir", "/realroot", os.FileMode(0755))

	mh.On("MustMount", "devtmpfs", "/dev", "devtmpfs", uintptr(syscall.MS_STRICTATIME|syscall.MS_NOSUID|syscall.MS_NOEXEC), "size=10M")

	mh.On("MustMount", "/dev/vda", "/ro", "squashfs", uintptr(syscall.MS_RDONLY), "")
	mh.On("MustMount", "/dev/vdb", "/rw", "ext4", uintptr(0), "")

	mh.On("MustMkdir", "/rw/upper", os.FileMode(0700))
	mh.On("MustMkdir", "/rw/work", os.FileMode(0700))
	mh.On("MustMount", "overlay-root", "/realroot", "overlay", uintptr(0), "lowerdir=/ro,upperdir=/rw/upper,workdir=/rw/work")

	mh.On("MustMkdir", "/realroot/ro", os.FileMode(0777))
	mh.On("MustMkdir", "/realroot/rw", os.FileMode(0777))

	mh.On("MustMount", "/ro", "/realroot/ro", "", uintptr(syscall.MS_MOVE), "")
	mh.On("MustMount", "/rw", "/realroot/rw", "", uintptr(syscall.MS_MOVE), "")

	mh.On("MustMkdir", "/realroot/dev", os.FileMode(0777))
	mh.On("MustMount", "/dev", "/realroot/dev", "", uintptr(syscall.MS_MOVE), "")

	mh.On("MustMkdir", "/realroot/proc", os.FileMode(0755))
	mh.On("MustMount", "proc", "/realroot/proc", "proc", uintptr(syscall.MS_NOSUID|syscall.MS_NODEV), "")

	mh.On("MustMkdir", "/realroot/sys", os.FileMode(0755))
	mh.On("MustMount", "sysfs", "/realroot/sys", "sysfs", uintptr(syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC), "")

	mh.On("MustMkdir", "/realroot/run", os.FileMode(0755))
	mh.On("MustMount", "tmpfs", "/realroot/run", "tmpfs", uintptr(syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_STRICTATIME), "size=20%")

	mh.On("MustMkdir", "/realroot/tmp", os.FileMode(0755))
	mh.On("MustMount", "tmpfs", "/realroot/tmp", "tmpfs", uintptr(syscall.MS_NOSUID|syscall.MS_NODEV), "size=50%")

	mh.On("MustMkdir", "/realroot/run/shm", os.FileMode(01777))
	mh.On("MustMount", "tmpfs", "/realroot/run/shm", "tmpfs", uintptr(syscall.MS_NOSUID|syscall.MS_NODEV|syscall.MS_NOEXEC|syscall.MS_STRICTATIME), "size=50%")

	mh.On("MustMkdir", "/realroot/sys/fs/cgroup", os.FileMode(0755))
	mh.On("MustMount", "tmpfs", "/realroot/sys/fs/cgroup", "tmpfs", uintptr(0), "size=1M")

	mh.On("MustMkdir", "/realroot/sys/fs/cgroup/systemd", os.FileMode(0755))
	mh.On("MustMount", "cgroup", "/realroot/sys/fs/cgroup/systemd", "cgroup", uintptr(0), "name=systemd,none")

	mh.On("MustMkdir", "/realroot/dev/pts", os.FileMode(0620))
	mh.On("MustMount", "devpts", "/realroot/dev/pts", "devpts", uintptr(syscall.MS_NOEXEC|syscall.MS_NOSUID), "ptmxmode=0666,gid=5,newinstance")

	mh.On("MustSymlink", "/proc/self/fd", "/realroot/dev/fd")
	mh.On("MustSymlink", "/proc/kcore", "/realroot/dev/core")
	mh.On("MustSymlink", "/proc/self/fd/0", "/realroot/dev/stdin")
	mh.On("MustSymlink", "/proc/self/fd/1", "/realroot/dev/stdout")
	mh.On("MustSymlink", "/proc/self/fd/2", "/realroot/dev/stderr")

	mh.On("MustChdir", "/realroot")

	mh.On("MustMount", "/realroot", "/", "", uintptr(syscall.MS_MOVE), "")

	mh.On("MustChroot", ".")

	mountAndPivotWithHelper(mh)

	mh.AssertExpectations(t)
}
