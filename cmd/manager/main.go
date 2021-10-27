package main

import (
	"firedocker/pkg/firecracker"
	"firedocker/pkg/networking"
	"fmt"
)

func main() {
	bnm, err := networking.InitializeBridgingNetworkManager("172.19.0.0/24")
	if err != nil {
		panic(err)
	}

	tapIf1, err := bnm.CreateTap()
	if err != nil {
		panic(err)
	}

	fmt.Printf("TAP1: name: %s idx: %d IP: %s MAC %s\n", tapIf1.Name(), tapIf1.Idx(), tapIf1.IP().String(), tapIf1.MAC())

	/*outfile, cfg, err := dockersquasher.PullAndSquash(dockersquasher.WithOutputFile("rootfs.sqs"), dockersquasher.WithTempDirectory("tmp"), dockersquasher.WithImage("ubuntu", "latest"))
	if err != nil {
		panic(err)
	}*/
	//fmt.Printf("Configuration: %+v\n", cfg)
	fmt.Println("rootfs done, starting VM")

	vmManager := firecracker.CreateManager()
	instance, err := vmManager.StartInstance()
	if err != nil {
		panic(err)
	}

	if err := instance.ConfigureAndStart(firecracker.Config{
		NetworkInterface:      tapIf1,
		RootFilesystemPath:    "./rootfs.sqs",
		ScratchFilesystemPath: "./scratch.ext4",
	}); err != nil {
		panic(err)
	}
	fmt.Println("Instance startup complete!")
	instance.Wait()
}
