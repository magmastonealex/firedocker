package main

import (
	"fmt"
	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func main() {

	ref, err := name.ParseReference("ubuntu:latest", name.WithDefaultRegistry("index.docker.io"))
	if err != nil {
		panic(err)
	}

	img, err := remote.Image(ref)
	if err != nil {
		panic(err)
	}
	manifest, err := img.Manifest()
	if err != nil {
		panic(err)
	}
	fmt.Printf("manifest: %+v\n", manifest)
	layers, err := img.Layers()
	for _, layer := range layers {
		fmt.Printf("layer: %+v\n", layer)
		mt, err := layer.MediaType()
		if err != nil {
			panic(err)
		}
		if mt != types.DockerLayer && mt != types.OCILayer {
			panic(fmt.Errorf("unknown layer type %+v", mt))
		}

		dg, err := layer.Digest()
		if err != nil {
			panic(err)
		}

		rc, err := layer.Compressed()
		if err != nil {
			panic(err)
		}
		outFile, err := os.Create(fmt.Sprintf("%s.tgz", dg.Hex))
		if err != nil {
			panic(err)
		}
		defer outFile.Close()
		defer rc.Close()
		_, err = io.Copy(outFile, rc)
		if err != nil {
			panic(err)
		}

		// extract, add our init & config, and run mksqashfs.
	}

	fmt.Printf("Here! %+v\n", img)
}
