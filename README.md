Firedocker
===

This is a tool to run Docker images as Firecracker microVMs.
It's similar in spirit to something like Ignite, but differs in a few key ways:
- Doesn't try to be a Docker-compatible.
- Much stronger enforcement of network sandboxing - using eBPF & TC (maybe eventually XDP...), VMs are restricted to the MAC and IP they were assigned, and all other traffic is dropped before the network stack even gets a chance to process it.
- Somewhat novel approach to building rootfs... The intended use case is situations where you'll run a particular image for a relatively long period of time. Images are converted to squashfs, and a small ext4 FS is mounted as an overlay on top for COW:
- Helper init which also acts as a syslogd instance.


**Note:** I'm building this as a hobby project. It's in no way ready for anyone else to use yet...
