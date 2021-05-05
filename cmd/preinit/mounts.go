package main

import (
	"os"
	"syscall"
)

type mountHelper interface {
	// MustMkdir wraps os.MkdirAll, and panics if the call fails.
	MustMkdir(path string, perm os.FileMode)
	// MustMount wraps syscall.Mount, and panics if the call fails.
	MustMount(src string, dst string, typ string, flags uintptr, data string)
	// MustSymlink wraps os.Symlink, and panics if the call fails.
	MustSymlink(file string, newname string)
	// MustChdir wraps unix.Chdir, and panics if the call fails.
	MustChdir(dir string)
	// MustChroot wraps unix.Chroot, and panics if the call fails.
	MustChroot(to string)
}

// MountAndPivot is responsible for setting up the real root fs and re-homing everything there.
// It will mount /dev/vda as the squashfs lower partition, and /dev/vdb as the upper partition.
// An overlay2 fs is created to provide the "real" root, and give the illusion of writable rootfs.
// Any of these calls failing is fatal - it leaves the system in an unknown state, and we can't recover.
// As such, failures will result in calls to Panic().
func MountAndPivot() {
	mountAndPivotWithHelper(&mountHelperImpl{})
}

func mountAndPivotWithHelper(mh mountHelper) {

	// Make some temp directories to mount things in.
	mh.MustMkdir("/ro", 0755)
	mh.MustMkdir("/rw", 0755)
	mh.MustMkdir("/realroot", 0755)

	// We need /dev to access the drives...
	var flags uintptr
	flags = syscall.MS_STRICTATIME | syscall.MS_NOSUID | syscall.MS_NOEXEC
	mh.MustMount("devtmpfs", "/dev", "devtmpfs", 0, "size=10M")

	// Mount our rootfs & writable area.
	flags = syscall.MS_RDONLY
	mh.MustMount("/dev/vda", "/ro", "squashfs", flags, "")
	mh.MustMount("/dev/vdb", "/rw", "ext4", 0, "")

	// Set up overlay...
	mh.MustMkdir("/rw/upper", 0700)
	mh.MustMkdir("/rw/work", 0700)
	mh.MustMount("overlay-root", "/realroot", "overlay", 0, "lowerdir=/ro,upperdir=/rw/upper,workdir=/rw/work")

	// and start moving things into realroot, where we'll create our rootfs.
	mh.MustMkdir("/realroot/ro", 0777)
	mh.MustMkdir("/realroot/rw", 0777)

	flags = syscall.MS_MOVE
	mh.MustMount("/ro", "/realroot/ro", "", flags, "")
	flags = syscall.MS_MOVE
	mh.MustMount("/rw", "/realroot/rw", "", flags, "")

	mh.MustMkdir("/realroot/dev", 0777)
	flags = syscall.MS_MOVE
	mh.MustMount("/dev", "/realroot/dev", "", flags, "")

	// Mount a number of API filesystems that normal programs expect to already be set up.
	// proc provides information about running processes
	mh.MustMkdir("/realroot/proc", 0755)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV
	mh.MustMount("proc", "/realroot/proc", "proc", flags, "")

	// sysfs provides all kinds of system information & kernel tuning.
	mh.MustMkdir("/realroot/sys", 0755)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC
	mh.MustMount("sysfs", "/realroot/sys", "sysfs", flags, "")

	// /run is a tmpdir
	mh.MustMkdir("/realroot/run", 0755)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_STRICTATIME
	mh.MustMount("tmpfs", "/realroot/run", "tmpfs", flags, "size=20%")

	// /tmp is a tmpdir
	mh.MustMkdir("/realroot/tmp", 0755)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV
	mh.MustMount("tmpfs", "/realroot/tmp", "tmpfs", flags, "size=50%")

	// /run/shm is a tmpdir
	mh.MustMkdir("/realroot/run/shm", 01777)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_STRICTATIME
	mh.MustMount("tmpfs", "/realroot/run/shm", "tmpfs", flags, "size=50%")

	// /sys/fs/cgroup is actually a tmpfs
	mh.MustMkdir("/realroot/sys/fs/cgroup", 0755)
	mh.MustMount("tmpfs", "/realroot/sys/fs/cgroup", "tmpfs", 0, "size=1M")

	// and a cgroup fs is mounted at  /sys/fs/cgroup/systemd for compatibility (we're not running actual systemd)
	mh.MustMkdir("/realroot/sys/fs/cgroup/systemd", 0755)
	mh.MustMount("cgroup", "/realroot/sys/fs/cgroup/systemd", "cgroup", 0, "name=systemd,none")

	// /dev/pty has information on pseudo-ttys
	mh.MustMkdir("/realroot/dev/pts", 0620)
	flags = syscall.MS_NOEXEC | syscall.MS_NOSUID
	mh.MustMount("devpts", "/realroot/dev/pts", "devpts", flags, "ptmxmode=0666,gid=5,newinstance")

	// A few symlinks are made to align with conventions
	mh.MustSymlink("/proc/self/fd", "/realroot/dev/fd")
	mh.MustSymlink("/proc/kcore", "/realroot/dev/core")
	mh.MustSymlink("/proc/self/fd/0", "/realroot/dev/stdin")
	mh.MustSymlink("/proc/self/fd/1", "/realroot/dev/stdout")
	mh.MustSymlink("/proc/self/fd/2", "/realroot/dev/stderr")

	// We're ready to move over to our new FS!
	mh.MustChdir("/realroot")

	// We're about to chroot, but we should clear out the old initramfs. Need an open fd to do that.
	// TODO: Actually clear out initramfs. It'd probably save a few MB of memory, and considering the size of these VMs,
	// actually pretty worthwhile.
	// You can do a trick where you grab a FD, switch away, then use the FD to delete itself.

	// move our real root over...
	flags = syscall.MS_MOVE
	mh.MustMount("/realroot", "/", "", flags, "")

	// and chroot into it to complete moving off of the initramfs.
	mh.MustChroot(".")
}
