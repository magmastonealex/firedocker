// Package dockersquasher provides a method of turning OCI Images from a Docker Registry
// into squashfs images. The host must provide `mksquashfs` and `tar` for this operation to succeed.
package dockersquasher

import (
	"firedocker/pkg/platformident"
	"fmt"
	"os/exec"

	"io"
	"os"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// HasRequiredUtilities ensures that required executables mksquashfs and tar are available.
// The main Squash function will also perform this check - it's provided as a convienience
// in case you want to pre-flight something.
func HasRequiredUtilities() error {
	_, err := exec.LookPath("tar")
	if err != nil {
		return fmt.Errorf("the 'tar' command is unavailable. cannot extract layers")
	}
	_, err = exec.LookPath("mksquashfs")
	if err != nil {
		return fmt.Errorf("the 'mksquashfs' command is unavailable. cannot ")
	}
	return nil
}

// PullAndSquash will attempt to fetch an image. By default, 'ubuntu:latest' will be fetched from index.docker.io,
// for the platform this binary was built for, using '.' as the temporary directory to use for storage.
// Pass SquashOptions to modify these defaults.
// TODO: Currently, this doesn't support authentication. It should be relatively easy to add, I just haven't needed it yet.
func PullAndSquash(configOptions ...SquashOption) (string, error) {
	config := pullSquashConfig{
		image:    "ubuntu",
		tag:      "latest",
		registry: "index.docker.io",
		forplat:  platformident.PlatformBuilt,
		tmpdir:   ".",
	}

	for _, option := range configOptions {
		option(&config)
	}

	err := HasRequiredUtilities()
	if err != nil {
		return "", fmt.Errorf("missing required utilities: %w", err)
	}

	ref, err := name.ParseReference(fmt.Sprintf("%s:%s", config.image, config.tag), name.WithDefaultRegistry(config.registry))
	if err != nil {
		return "", fmt.Errorf("image, tag, or registry is invalid: %w", err)
	}

	imgIndex, err := remote.Index(ref)
	if err != nil {
		return "", fmt.Errorf("failed to find image %s:%s @ %s : %w", config.image, config.tag, config.registry, err)
	}
	manifest, err := imgIndex.IndexManifest()
	if err != nil {
		return "", fmt.Errorf("failed to parse image manifest: %w", err)
	}

	manifestAvailable := make(map[string]containerregistry.Hash)
	for _, mani := range manifest.Manifests {
		// Try to find a suitable manifest...
		if mani.Platform.OS != "linux" {
			continue
		}
		manifestAvailable[fmt.Sprintf("%s%s", mani.Platform.Architecture, mani.Platform.Variant)] = mani.Digest
	}
	var suitableManifest containerregistry.Hash
	// Find the most suitable manifest...
	if config.forplat == platformident.PlatformX86_64 {
		if val, ok := manifestAvailable["amd64"]; ok {
			suitableManifest = val
		} else if val, ok := manifestAvailable["386"]; ok {
			suitableManifest = val
		}
	} else if config.forplat == platformident.PlatformAArch64 {
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
		return "", fmt.Errorf("unknown platform type %v", config.forplat)
	}

	if suitableManifest.Hex == "" {
		return "", fmt.Errorf("no suitable image found - try another platform")
	}

	ref, err = name.ParseReference(fmt.Sprintf("%s@%s:%s", config.image, suitableManifest.Algorithm, suitableManifest.Hex), name.WithDefaultRegistry(config.registry))
	if err != nil {
		return "", fmt.Errorf("failed to create reference to image: %w", err)
	}

	img, err := remote.Image(ref)
	if err != nil {
		return "", fmt.Errorf("failed to retrieve image manifest: %w", err)
	}

	imgManifest, err := img.RawManifest()
	if err != nil {
		return "", fmt.Errorf("failed to read manifest from image: %w", err)
	}
	panic(fmt.Errorf("Haven't finished converting the rest yet. Try again tomorrow."))
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
		tarExtract := exec.Command("tar", "-xpzf", "-", "-C", "workdir/")
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

	return "", nil
}
