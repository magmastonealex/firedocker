Firedocker
===

This is a tool to run Docker images as Firecracker microVMs.

The end goal is to build a framework allowing you to relatively securely run untrusted multi-tenanted workloads on your own hardware.

It's similar in spirit to something like Ignite, allowing running Docker images as Firecracker microVMs. Some key differences:

- Doesn't try to be a Docker-compatible. It processes docker images into VM images, but does not use the docker CLI for management
- Much stronger enforcement of network sandboxing - using eBPF & TC, VMs are restricted to the MAC and IP they were assigned, and all other traffic is dropped before the network stack even gets a chance to process it.
- Somewhat novel approach to building rootfs... The intended use case is situations where you'll run a particular image for a relatively long period of time. Images are converted to squashfs, and a small ext4 FS is mounted as an overlay on top for COW.
- More useful init which also acts as a syslogd instance.
- Strong control over resource allocations.
- Scheme for credentialling VMs and authenticating to services running on the host system.
- Pure Go implementation & fewer runtime configuration requirements.

**Note:** I'm building this as a hobby project. It's in no way ready for anyone else to use yet...

Big TODOs:

- ~~Proof-Of-Concept - eBPF isolation, docker images to squashfs, golang init that can set everything up for a working system, overlayfs RW root, works on arm64 & x86_64~~ Done! Now to make it real.
- ~~Docker image squashing into squashfs img as indepedent tested package~~
- ~~eBPF isolation implemented in it's own tested package~~
- ~~Basic init implemented as it's own binary~~ - still need to implement config retrieval & reaping. Dependent on config interfaces.
- ~Building package to handle setting up network bridge, TAP devices in netns~
- Simple VM booting from the manager.
- VSock interface allowing communication between manager and various init processes.
- Init accepts a configuration & can start the main process and optionally an SSH server.
- Init reports logs back to the manager.
- VM booting using `jailer`, integration with network management.
- Describe & implement configuration file or interface for the manager.
- Allow the manager to manage a number of different VMs and auto-restart as needed.
- Resource quotas - firecracker gives tools to limit I/O, CPU, and memory. Take advantage and actually set those options.
- Manager issues JWTs and makes them available to containers via the metadata service
- Figure out packaging & document dependencies and how someone _else_ could set this up.
- Architecture documents

**Building aarch64 kernel example**:

- *Note*: On x86_64, the firecracker 'default' kernel from the getting started guide is "good enough" - just not quite new enough for the random kernel config. This is just guide to build for aarch64.
- Download your favourite kernel. `wget https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-5.12.1.tar.xz`
- Extract... `tar -xJvf linux-5.12.1.tar.xz`
- You can use whatever kconfig you want. https://raw.githubusercontent.com/firecracker-microvm/firecracker/main/resources/microvm-kernel-arm64.config is a good start. Copy to .config. There's one for 5.12.1 in aarch-kconfig in this repo.
- I _think_ you can use almost any aarch64 compiler you want, but Ubuntu's gcc-aarch64-linux-gnu seems to be okay.
- make oldconfig
- apt-get install flex bison gcc-aarch64-linux-gnu bc
- `make  ARCH=arm64 CROSS_COMPILE=aarch64-linux-gnu- oldconfig`
- `make  ARCH=arm64 CROSS_COMPILE=aarch64-linux-gnu- Image`
- Grab the Image from arch/arm64/boot/Image, rather than vmlinux
- Your Image is in arch/arm


