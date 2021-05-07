// Package dockersquasher provides a method of turning OCI Images from a Docker Registry
// into squashfs images. The host must provide `mksquashfs` and `tar` for this operation to succeed.
package dockersquasher

import (
	"firedocker/pkg/platformident"
	"fmt"
	"path"

	"os"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// PullAndSquash will attempt to fetch an image. By default, 'ubuntu:latest' will be fetched from index.docker.io,
// for the platform this binary was built for, using '.' as the temporary directory to use for storage.
// Pass SquashOptions to modify these defaults.
// TODO: Currently, this doesn't support authentication. It should be relatively easy to add, I just haven't needed it yet.
func PullAndSquash(configOptions ...SquashOption) (string, *containerregistry.ConfigFile, error) {
	return pullAndSquashWithRemote(remoteRepositoryImpl{}, tarSquasherImpl{}, configOptions...)
}

func pullAndSquashWithRemote(repo remoteRepository, tarSquash tarSquasher, configOptions ...SquashOption) (string, *containerregistry.ConfigFile, error) {
	config := pullSquashConfig{
		image:    "ubuntu",
		tag:      "latest",
		registry: "index.docker.io",
		forplat:  platformident.PlatformBuilt,
		tmpdir:   ".",
		outfile:  "./img.sqfs",
	}

	for _, option := range configOptions {
		option(&config)
	}

	ref, err := name.ParseReference(fmt.Sprintf("%s:%s", config.image, config.tag), name.WithDefaultRegistry(config.registry))
	if err != nil {
		return "", nil, fmt.Errorf("image, tag, or registry is invalid: %w", err)
	}

	imgIndex, err := repo.Index(ref)
	if err != nil {
		return "", nil, fmt.Errorf("failed to find image %s:%s @ %s : %w", config.image, config.tag, config.registry, err)
	}
	manifest, err := imgIndex.IndexManifest()
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse image manifest: %w", err)
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
		return "", nil, fmt.Errorf("unknown platform type %v", config.forplat)
	}

	if suitableManifest.Hex == "" {
		return "", nil, fmt.Errorf("no suitable image found - try another platform")
	}

	ref, err = name.ParseReference(fmt.Sprintf("%s@%s:%s", config.image, suitableManifest.Algorithm, suitableManifest.Hex), name.WithDefaultRegistry(config.registry))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create reference to image: %w", err)
	}

	img, err := repo.Image(ref)
	if err != nil {
		return "", nil, fmt.Errorf("failed to retrieve image manifest: %w", err)
	}

	configFile, err := img.ConfigFile()
	if err != nil {
		return "", nil, fmt.Errorf("failed to retrieve configuration file for image %w", err)
	}

	layers, err := img.Layers()
	if err != nil {
		return "", nil, fmt.Errorf("image has no layers? %w", err)
	}
	workdir := path.Join(config.tmpdir, "squashwork")
	os.RemoveAll(workdir)
	os.Mkdir(workdir, 0700)
	defer os.RemoveAll(workdir)
	for _, layer := range layers {
		dg, err := layer.Digest()
		if err != nil {
			return "", nil, fmt.Errorf("layer has no digest!? %w", err)
		}

		mt, err := layer.MediaType()
		if err != nil {
			return "", nil, fmt.Errorf("layer %s has no media type: %w", dg.Hex, err)
		}
		if mt != types.DockerLayer && mt != types.OCILayer {
			return "", nil, fmt.Errorf("unknown layer type %+v for %s", mt, dg.Hex)
		}

		rc, err := layer.Compressed()
		if err != nil {
			return "", nil, fmt.Errorf("failed to start download of layer %s: %w", dg.Hex, err)
		}

		// Extract this into workdir...
		err = tarSquash.Extract(rc, workdir)
		if err != nil {
			return "", nil, fmt.Errorf("tar failed to extract layer. %w", err)
		}
	}

	err = tarSquash.Squash(workdir, config.outfile)
	if err != nil {
		return "", nil, fmt.Errorf("failed to squash the rootfs. %w", err)
	}

	return config.outfile, configFile, nil
}
