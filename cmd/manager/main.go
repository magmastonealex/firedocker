package main

import (
	"firedocker/pkg/dockersquasher"
	"firedocker/pkg/firecracker"
	"firedocker/pkg/networking"
	"firedocker/pkg/storagemanager"
	"fmt"
)

func main() {
	bnm, err := networking.InitializeBridgingNetworkManager("172.19.0.0/24")
	if err != nil {
		panic(err)
	}

	storage := storagemanager.CreateRawStorageManager("./scratch")

	numVms := 1

	tapInterfaces := make([]networking.TAPInterface, numVms)
	vms := make([]firecracker.VMInstance, numVms)

	for i := range tapInterfaces {
		tapInterfaces[i], err = bnm.CreateTap()
		if err != nil {
			panic(err)
		}
	}

	outfile, cfg, err := dockersquasher.PullAndSquash(dockersquasher.WithOutputFile("rootfs.sqs"), dockersquasher.WithTempDirectory("tmp"), dockersquasher.WithImage("redis", "latest"))
	if err != nil {
		panic(err)
	}

	//fmt.Printf("Configuration: %+v\n", cfg)
	fmt.Println("rootfs done, starting VM")

	vmManager := firecracker.CreateManager()

	for i := range vms {
		vms[i], err = vmManager.StartInstance()
		if err != nil {
			panic(err)
		}
		scratchPath, err := storage.CreateFilesystemImage(vms[i].ID(), 200)
		if err != nil {
			panic(err)
		}
		if err := vms[i].ConfigureAndStart(firecracker.Config{
			NetworkInterface:      tapInterfaces[i],
			RootFilesystemPath:    outfile,
			ScratchFilesystemPath: scratchPath,
			RuntimeConfig: firecracker.ContainerRuntimeConfig{
				Environment: cfg.Config.Env,
				Entrypoint:  cfg.Config.Entrypoint,
				Cmd:         cfg.Config.Cmd,
				Workdir:     cfg.Config.WorkingDir,
			},
		}); err != nil {
			panic(err)
		}
	}

	fmt.Println("Instance startup complete!")
	for i := range vms {
		vms[i].Wait()
	}
}
