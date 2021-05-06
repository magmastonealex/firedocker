package main

import "firedocker/pkg/dockersquasher"

func main() {
	imgToPull := "ubuntu"
	tagToPull := "latest"

	res, err := dockersquasher.PullAndSquash(dockersquasher.WithImage("debian", "latest"))
	if err != nil {
		panic(err)
	}

}
