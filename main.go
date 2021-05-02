package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func main() {
	imgToPull := "redis"
	tagToPull := "latest"

	ref, err := name.ParseReference(fmt.Sprintf("%s:%s", imgToPull, tagToPull), name.WithDefaultRegistry("index.docker.io"))
	if err != nil {
		panic(err)
	}

	imgIndex, err := remote.Index(ref)
	if err != nil {
		panic(err)
	}
	manifest, err := imgIndex.IndexManifest()
	if err != nil {
		panic(err)
	}

	manifestAvailable := make(map[string]containerregistry.Hash)
	for _, mani := range manifest.Manifests {
		fmt.Printf("manifest: %s Platform %s %s %s\n", mani.Digest.Hex, mani.Platform.Architecture, mani.Platform.Variant, mani.Platform.OS)
		// Try to find a suitable manifest...
		if mani.Platform.OS != "linux" {
			fmt.Printf("Not a Linux image... Skipping.\n")
			continue
		}
		manifestAvailable[fmt.Sprintf("%s%s", mani.Platform.Architecture, mani.Platform.Variant)] = mani.Digest
	}
	var suitableManifest containerregistry.Hash
	// Find the most suitable manifest...
	if PlatformBuilt == PlatformX86_64 {
		if val, ok := manifestAvailable["amd64"]; ok {
			suitableManifest = val
		} else if val, ok := manifestAvailable["386"]; ok {
			suitableManifest = val
		}
	} else if PlatformBuilt == PlatformAArch64 {
		// From highest to lowest priority.
		// We can run all of these, but 64-bit images should be preferred.
		if val, ok := manifestAvailable["arm64v8"]; ok {
			suitableManifest = val
		} else if val, ok := manifestAvailable["armv8"]; ok {
			suitableManifest = val
		} else if val, ok := manifestAvailable["armv7"]; ok {
			suitableManifest = val
		} else if val, ok := manifestAvailable["armv5"]; ok {
			suitableManifest = val
		} else if val, ok := manifestAvailable["armv8"]; ok {
			suitableManifest = val
		}
	} else {
		panic(fmt.Errorf("unknown platform type"))
	}

	if suitableManifest.Hex == "" {
		panic(fmt.Errorf("no suitable image found"))
	}

	ref, err = name.ParseReference(fmt.Sprintf("%s@%s:%s", imgToPull, suitableManifest.Algorithm, suitableManifest.Hex), name.WithDefaultRegistry("index.docker.io"))
	if err != nil {
		panic(err)
	}

	img, err := remote.Image(ref)
	if err != nil {
		panic(err)
	}

	imgManifest, err := img.RawManifest()
	if err != nil {
		panic(err)
	}
	fmt.Printf("manifest: %+s\n", string(imgManifest))
	layers, err := img.Layers()
	os.RemoveAll("workdir")
	os.Mkdir("workdir", 0700)
	for _, layer := range layers {
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
		fmt.Printf("Dl: %s\n", dg.Hex)

		rc, err := layer.Compressed()
		if err != nil {
			panic(err)
		}
		// Extract this into workdir...
		tarExtract := exec.Command("tar", "-xzf", "-", "-C", "workdir/")
		tarErr, _ := tarExtract.StderrPipe()
		tarOut, _ := tarExtract.StdoutPipe()
		tarIn, _ := tarExtract.StdinPipe()
		err = tarExtract.Start()
		go func() {
			io.Copy(os.Stdout, tarOut)
		}()
		go func() {
			io.Copy(os.Stderr, tarErr)
		}()
		if err != nil {
			panic(err)
		}
		_, err = io.Copy(tarIn, rc)
		if err != nil {
			panic(err)
		}
		rc.Close()
		tarIn.Close()
		err = tarExtract.Wait()
		if err != nil {
			panic(err)
		}
	}

	mksqfs := exec.Command("mksquashfs", "workdir", fmt.Sprintf("%s_%s.sqfs", imgToPull, tagToPull))
	mksqfsErr, _ := mksqfs.StderrPipe()
	mksqfsOut, _ := mksqfs.StdoutPipe()
	err = mksqfs.Start()
	if err != nil {
		panic(err)
	}
	go func() {
		io.Copy(os.Stdout, mksqfsOut)
	}()
	go func() {
		io.Copy(os.Stderr, mksqfsErr)
	}()
	err = mksqfs.Wait()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Here! %+v\n", img)

}
