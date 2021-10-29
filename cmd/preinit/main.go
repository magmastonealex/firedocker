package main

// build w/ netgo
// go build -tags netgo
// find . -print0 | cpio --null --create --verbose --format=newc > ../initrd.cpio

import (
	"firedocker/cmd/preinit/mmds"
	"firedocker/cmd/preinit/netsettings"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// This will get run as init in the initramfs (and be the only binary in there)
// https://github.com/tsirakisn/u-root/blob/26a90287872f42e357dc889f6918855fc0fde4dc/pkg/mount/switch_root_linux.go#L104
// It will setup overlay, and then pivot into the new COW filesystem.
// Then, /bin/mio-init will be invoked.
func main() {
	fmt.Println("I'm init! Mounting")

	MountAndPivot()

	fmt.Println("Querying MMDS for IP configuration")

	// Apply an early net config to talk to MMDS.
	// Doesn't really need to be correct, we're banned from sending
	// invalid settings via the eBPF filtering anyways.
	err := netsettings.ApplyNetConfig("eth0", netsettings.NetConfig{
		IPNet: "169.254.169.3/24",
	})
	if err != nil {
		panic(fmt.Errorf("failed to set initial network: %w", err))
	}

	mmdsConfig, err := mmds.FetchIPConfig()
	if err != nil {
		panic(fmt.Errorf("failed to retrieve IP configuration: %w", err))
	}

	runtimeConfig, err := mmds.FetchRuntimeConfig()
	if err != nil {
		panic(fmt.Errorf("failed to retrieve runtime configuration: %w", err))
	}

	routeConfig := make([]netsettings.RouteConfig, len(mmdsConfig.Routes))
	for i, route := range mmdsConfig.Routes {
		routeConfig[i] = netsettings.RouteConfig{
			Gw:  route.Gw,
			Dst: route.Network,
		}
	}

	fmt.Printf("Setting IP to %s\n", mmdsConfig.IPCIDR)
	err = netsettings.ApplyNetConfig("eth0", netsettings.NetConfig{
		IPNet:  mmdsConfig.IPCIDR,
		Routes: routeConfig,
	})
	if err != nil {
		panic(fmt.Errorf("failed to set up networking: %w", err))
	}

	fmt.Println("Setting up resolv.conf")
	resolvconf := []byte(fmt.Sprintf("nameserver %s\nnameserver %s\n", mmdsConfig.PrimaryDNS, mmdsConfig.SecondaryDNS))
	if err := os.WriteFile("/etc/resolv.conf", resolvconf, 0o644); err != nil {
		panic(fmt.Errorf("failed to set resolv.conf"))
	}

	// We're ready to start invoking programs now.
	// Let's set the basic env vars...

	if err := os.Setenv("PATH", "/usr/local/bin:/usr/local/sbin:/usr/bin:/usr/sbin:/bin:/sbin"); err != nil {
		panic(fmt.Errorf("Failed to set PATH!? %v", err))
	}

	if err := os.Setenv("LANG", "C.UTF-8"); err != nil {
		panic(fmt.Errorf("Failed to set PATH!? %v", err))
	}

	for _, env := range runtimeConfig.Environment {
		envSetting := strings.SplitN(env, "=", 2)
		if err := os.Setenv(envSetting[0], envSetting[1]); err != nil {
			panic(fmt.Errorf("Failed to set %s!? %v", envSetting[0], err))
		}
	}

	execArgs := runtimeConfig.Entrypoint
	// Otherwise, start the entrypoint for the container.
	if len(runtimeConfig.Entrypoint) > 0 {
		execArgs = append(execArgs, runtimeConfig.Cmd...)
	} else {
		execArgs = runtimeConfig.Cmd
	}

	fmt.Printf("Execing... %+v\n", execArgs)

	os.Chdir(runtimeConfig.Workdir)

	cmd := exec.Command(execArgs[0], execArgs[1:]...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(fmt.Errorf("failed to get stdout: %w", err))
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(fmt.Errorf("failed to get stderr: %w", err))
	}

	go io.CopyBuffer(os.Stdout, stdout, make([]byte, 255))
	go io.CopyBuffer(os.Stdout, stderr, make([]byte, 255))

	cmd.Start()
	// If we're in dev mode, spawn the SSH server by re-invoking ourselves
	err = StartServer()
	if err != nil {
		panic(fmt.Errorf("failed to start ssh server: %v", err))
	}

	// Connect up to the manager over vsock
	// Retrieve this VM's configuration via manager RPC method
	// Set hostname (should eventually be based on config...)
	// Set resolv.conf
	// Set up signal handlers.
	//
	// We've done most of our exec-ing...
	// Start reaping child processes who were re-parented.
	// From here on out, you have to be careful when you spawn programs - you can't
	// use os/exec and expect to get the return code, since this loop will be Wait4-ing on everything.
	// If you start a process, you'll have to clean it up from this loop.
	// Most processes are just waited on and discarded. Two processes are special cases:
	//  - The entrypoint for this container - exiting will cause the VM to exit.
	//  - If enabled, the debug SSH server (aka this process with different args) - exiting will cause it to be re-spawned.
}
