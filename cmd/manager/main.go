package main

import (
	"firedocker/pkg/packetfilter"
	"fmt"
)

func main() {

	bpfMap, err := packetfilter.OpenMap("/sys/fs/bpf/tc/globals/ifce_allowed_ip")
	if err != nil {
		panic(err)
	}
	fmt.Println("Finished!?")

	val, err := bpfMap.GetValue(16)
	if err != nil {
		panic(err)
	}

	fmt.Printf("result was: %x", val)

}
