package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/unix"
)

func MustMkdir(path string, perm os.FileMode) {
	err := os.MkdirAll(path, perm)
	if err != nil {
		panic(err)
	}
}

func MustMount(src string, dst string, typ string, flags uintptr, data string) {
	err := syscall.Mount(src, dst, typ, flags, data)
	if err != nil {
		panic(err)
	}
}

func MustSymlink(file string, newname string) {
	err := os.Symlink(file, newname)
	if err != nil {
		panic(err)
	}
}

// This will get run as init in the initramfs (and be the only binary in there)
// https://github.com/tsirakisn/u-root/blob/26a90287872f42e357dc889f6918855fc0fde4dc/pkg/mount/switch_root_linux.go#L104
// It will setup overlay, and then pivot into the new COW filesystem.
// Then, /bin/mio-init will be invoked.
func main() {
	fmt.Println("I'm init! Mounting ")

	MustMkdir("/ro", 0755)
	MustMkdir("/rw", 0755)
	MustMkdir("/realroot", 0755)

	// We need /dev to access the drives...
	fmt.Println("Mounting devtmpfs...")
	var flags uintptr
	flags = syscall.MS_STRICTATIME | syscall.MS_NOSUID | syscall.MS_NOEXEC
	MustMount("devtmpfs", "/dev", "devtmpfs", 0, "size=10M")

	// Mount our rootfs & writable area.
	flags = syscall.MS_RDONLY
	MustMount("/dev/vda", "/ro", "squashfs", flags, "")
	MustMount("/dev/vdb", "/rw", "ext4", 0, "")

	// Set up overlay...
	MustMkdir("/rw/upper", 0700)
	MustMkdir("/rw/work", 0700)

	MustMount("overlay-root", "/realroot", "overlay", 0, "lowerdir=/ro,upperdir=/rw/upper,workdir=/rw/work")

	// and start moving things into realroot...
	MustMkdir("/realroot/ro", 0777)
	MustMkdir("/realroot/rw", 0777)

	flags = syscall.MS_MOVE
	MustMount("/ro", "/realroot/ro", "", flags, "")
	flags = syscall.MS_MOVE
	MustMount("/rw", "/realroot/rw", "", flags, "")

	MustMkdir("/realroot/dev", 0777)
	flags = syscall.MS_MOVE
	MustMount("/dev", "/realroot/dev", "", flags, "")

	MustMkdir("/realroot/proc", 0755)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV
	MustMount("proc", "/realroot/proc", "proc", flags, "")

	MustMkdir("/realroot/sys", 0755)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC
	MustMount("sysfs", "/realroot/sys", "sysfs", flags, "")

	MustMkdir("/realroot/run", 0755)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_STRICTATIME
	MustMount("tmpfs", "/realroot/run", "tmpfs", flags, "size=20%")

	MustMkdir("/realroot/tmp", 0755)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV
	MustMount("tmpfs", "/realroot/tmp", "tmpfs", flags, "size=50%")

	MustMkdir("/realroot/run/shm", 01777)
	flags = syscall.MS_NOSUID | syscall.MS_NODEV | syscall.MS_NOEXEC | syscall.MS_STRICTATIME
	MustMount("tmpfs", "/realroot/run/shm", "tmpfs", flags, "size=50%")

	MustMkdir("/realroot/sys/fs/cgroup", 0755)
	MustMount("tmpfs", "/realroot/sys/fs/cgroup", "tmpfs", 0, "size=1M")

	MustMkdir("/realroot/sys/fs/cgroup/systemd", 0755)
	MustMount("cgroup", "/realroot/sys/fs/cgroup/systemd", "cgroup", 0, "name=systemd,none")

	MustMkdir("/realroot/dev/pts", 0620)
	flags = syscall.MS_NOEXEC | syscall.MS_NOSUID
	MustMount("devpts", "/realroot/dev/pts", "devpts", flags, "ptmxmode=0666,gid=5,newinstance")

	MustSymlink("/proc/self/fd", "/realroot/dev/fd")
	MustSymlink("/proc/kcore", "/realroot/dev/core")
	MustSymlink("/proc/self/fd/0", "/realroot/dev/stdin")
	MustSymlink("/proc/self/fd/1", "/realroot/dev/stdout")
	MustSymlink("/proc/self/fd/2", "/realroot/dev/stderr")

	fmt.Printf("New root on /realroot is ready! Switching there...")
	if err := unix.Chdir("/realroot"); err != nil {
		panic(fmt.Errorf("failed change directory to new_root %v", err))
	}

	if err := unix.Chdir("/realroot"); err != nil {
		panic(fmt.Errorf("failed change directory to /realroot %v", err))
	}

	// We're about to chroot, but we should clear out the old initramfs. Need an open fd to do that.
	// TODO: Actually clear out initramfs. It'd probably save a few MB of memory, and considering the size of these VMs,
	// actually pretty worthwhile.
	// You can do a trick where you grab a FD, switch away, then use the FD to delete itself.

	// move our real root over...
	flags = syscall.MS_MOVE
	MustMount("/realroot", "/", "", flags, "")

	// and chroot.
	if err := unix.Chroot("."); err != nil {
		panic(fmt.Errorf("failed to chroot %v", err))
	}

	// And set some sane PATH & LANG settings for now...
	if err := os.Setenv("PATH", "/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin"); err != nil {
		panic(fmt.Errorf("Failed to set PATH!? %v", err))
	}

	if err := os.Setenv("LANG", "C.UTF-8"); err != nil {
		panic(fmt.Errorf("Failed to set PATH!? %v", err))
	}

	fmt.Println("Init done. Setsid, away!")
	// Let's detach ourselves and see if we can spawn a getty...
	if _, err := syscall.Setsid(); err != nil {
		panic(fmt.Errorf("failed to enter new session ID: %v", err))
	}
	/*
		fmt.Println("Setting IP to 172.19.0.2/24")
		ifce, err := tenus.NewLinkFrom("tap0")
		if err != nil {
			panic(fmt.Errorf("failed to find ifce: %v", err))
		}

		network := &net.IPNet{
			IP:   net.ParseIP("172.19.0.2"),
			Mask: net.CIDRMask(24, 32),
		}
		err = ifce.SetLinkIp(network.IP, network)
		if err != nil {
			panic(fmt.Errorf("failed to ste IP: %v", err))
		}

		gateway := net.ParseIP("172.19.0.1")
		err = ifce.SetLinkDefaultGw(&gateway)
		if err != nil {
			panic(fmt.Errorf("failed to set GW: %v", err))
		}*/

	os.Stdout.Close()
	os.Stderr.Close()
	os.Stdin.Close()

	fmt.Println("and now getty...")

	mksqfs := exec.Command("/usr/sbin/getty", "/dev/ttyS0")
	if err := mksqfs.Start(); err != nil {
		panic(err)
	}

	if err := mksqfs.Wait(); err != nil {
		panic(err)
	}

	// https://landley.net/writing/rootfs-programming.html

	// Mount overlayfs  similar to https://gist.github.com/mutability/6cc944bde1cf4f61908e316befd42bc4
	// Pivot over to the new FS.
}
