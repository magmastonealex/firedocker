package main

import (
	"firedocker/pkg/bpfmap"
	"firedocker/pkg/networking"
	"fmt"
)

func main() {
	bnm, err := networking.InitializeBridgingNetworkManager("192.168.3.0/24")
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

	bpfMap, err := bpfmap.OpenMap("/sys/fs/bpf/tc/globals/ifce_allowed_ip")
	if err != nil {
		panic(err)
	}
	fmt.Println("Finished!?")

	val, err := bpfMap.GetCurrentValues()
	if err != nil {
		panic(err)
	}

	fmt.Printf("result was: %+v\n", val)

}
