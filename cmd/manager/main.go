package main

import (
	"firedocker/pkg/packetfilter"
)

func main() {
	/*
		bpfMap, err := bpfmap.OpenMap("/sys/fs/bpf/tc/globals/ifce_allowed_ip")
		if err != nil {
			panic(err)
		}
		fmt.Println("Finished!?")

		err = bpfMap.DeleteValue(35)
		if err != nil {
			panic(err)
		}

		val, err := bpfMap.GetCurrentValues()
		if err != nil {
			panic(err)
		}

		fmt.Printf("result was: %+v\n", val)

		bpfMap.SetValue(35, 323123)

		val, err = bpfMap.GetCurrentValues()
		if err != nil {
			panic(err)
		}

		fmt.Printf("result was: %+v\n", val)
	*/
	wl := packetfilter.DefaultPacketWhitelister{}
	wl.UpdateByIndex(3, "172.19.0.2", "aa:bb:cc:dd:ee:ff")
}
