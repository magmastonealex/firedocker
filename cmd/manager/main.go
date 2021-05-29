package main

import (
	"firedocker/pkg/networking"
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
	_, err := networking.InitializeBridgingNetworkManager("192.168.3.0/24", "10.121.43.8/31")
	if err != nil {
		panic(err)
	}
}
