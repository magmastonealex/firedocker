package main

import (
	"firedocker/pkg/dockersquasher"
	"fmt"
)

func main() {

	res, cfg, err := dockersquasher.PullAndSquash(dockersquasher.WithImage("debian", "latest"))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Resulting filename: %s\n", res)
	fmt.Printf("entry: %+v\n", cfg.Config.Entrypoint)

}
