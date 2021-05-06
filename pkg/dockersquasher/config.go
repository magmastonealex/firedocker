package dockersquasher

import "firedocker/pkg/platformident"

type pullSquashConfig struct {
	image    string
	tag      string
	registry string
	forplat  platformident.PlatformVariant
	tmpdir   string
}

// SquashOption is a functional option for squashing images.
type SquashOption func(*pullSquashConfig)

// WithImage sets the image to retrive
func WithImage(img string, tag string) SquashOption {
	return func(config *pullSquashConfig) {
		config.image = img
		config.tag = tag
	}
}

// WithRegistry sets the registry that the image should be retrived from.
func WithRegistry(registry string) SquashOption {
	return func(config *pullSquashConfig) {
		config.registry = registry
	}
}

// WithPlatform sets the platform for which the image should be downloaded.
func WithPlatform(plat platformident.PlatformVariant) SquashOption {
	return func(config *pullSquashConfig) {
		config.forplat = plat
	}
}

// WithTempDirectory sets the directory that will be used for storage - a subdirectory will be created
// underneath, so no need to clear it out.
func WithTempDirectory(dir string) SquashOption {
	return func(config *pullSquashConfig) {
		config.tmpdir = dir
	}
}
