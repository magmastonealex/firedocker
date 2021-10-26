package main

import (
	"firedocker/pkg/dockersquasher"
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
	tapIf2, err := bnm.CreateTap()
	if err != nil {
		panic(err)
	}

	fmt.Printf("TAP1: name: %s idx: %d IP: %s MAC %s\n", tapIf1.Name(), tapIf1.Idx(), tapIf1.IP().String(), tapIf1.MAC())
	fmt.Printf("TAP2: name: %s idx: %d IP: %s MAC %s\n", tapIf2.Name(), tapIf2.Idx(), tapIf2.IP().String(), tapIf2.MAC())

	outfile, cfg, err := dockersquasher.PullAndSquash(dockersquasher.WithOutputFile("rootfs.sqs"), dockersquasher.WithTempDirectory("tmp"), dockersquasher.WithImage("ubuntu", "latest"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Configuration: %+v\n", cfg)
	fmt.Printf("rootfs in %s\n", outfile)

}
