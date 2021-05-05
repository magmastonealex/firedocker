Firedocker
===

This is a tool to run Docker images as Firecracker microVMs. The end goal is to build a framework allowing you to relatively securely run untrusted multi-tenanted workloads on your own hardware.


It's similar in spirit to something like Ignite, but differs in a few key ways:
- Doesn't try to be a Docker-compatible.
- Much stronger enforcement of network sandboxing - using eBPF & TC (maybe eventually XDP...), VMs are restricted to the MAC and IP they were assigned, and all other traffic is dropped before the network stack even gets a chance to process it.
- Somewhat novel approach to building rootfs... The intended use case is situations where you'll run a particular image for a relatively long period of time. Images are converted to squashfs, and a small ext4 FS is mounted as an overlay on top for COW:
- Helper init which also acts as a syslogd instance.


Big TODOs:
    - It's all a hack! I'm trying to prove out viability. None of this is decent code yet. Needs a lot of refactoring and architectural thinking.

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


**Note:** I'm building this as a hobby project. It's in no way ready for anyone else to use yet...
