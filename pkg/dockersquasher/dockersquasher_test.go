package dockersquasher

//go:generate mockery --name=remoteRepository --structname=RemoteRepository

//go:generate mockery --name=tarSquasher --structname=TarSquasher

import (
	"bytes"
	"firedocker/pkg/dockersquasher/mocks"
	"firedocker/pkg/platformident"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

type fakeLayer struct {
	id string // everything else is synthesized based on this. Set it to whatever you want.
}

func (fl *fakeLayer) Digest() (containerregistry.Hash, error) {
	return containerregistry.Hash{
		Hex:       fmt.Sprintf("%s_comp", fl.id),
		Algorithm: "shafake1",
	}, nil
}

func (fl *fakeLayer) DiffID() (containerregistry.Hash, error) {
	return containerregistry.Hash{
		Hex:       fmt.Sprintf("%s_uncom", fl.id),
		Algorithm: "shafake2",
	}, nil
}

func (fl *fakeLayer) Compressed() (io.ReadCloser, error) {
	return ioutil.NopCloser(bytes.NewReader([]byte(fmt.Sprintf("layerIs:%s", fl.id)))), nil
}

func (fl *fakeLayer) Uncompressed() (io.ReadCloser, error) {
	return nil, nil
}

func (fl *fakeLayer) Size() (int64, error) {
	return 100, nil
}

func (fl *fakeLayer) MediaType() (types.MediaType, error) {
	return types.DockerLayer, nil
}

type fakeImage struct {
	layers []containerregistry.Layer
	config *containerregistry.ConfigFile
}

func (fi *fakeImage) Layers() ([]containerregistry.Layer, error) {
	return fi.layers, nil
}

func (fi *fakeImage) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema2, nil
}

func (fi *fakeImage) Size() (int64, error) {
	return 100, nil
}

func (fi *fakeImage) ConfigName() (containerregistry.Hash, error) {
	return containerregistry.Hash{}, nil
}

func (fi *fakeImage) ConfigFile() (*containerregistry.ConfigFile, error) {
	return fi.config, nil
}

func (fi *fakeImage) RawConfigFile() ([]byte, error) {
	return []byte{}, nil
}

func (fi *fakeImage) Digest() (containerregistry.Hash, error) {
	return containerregistry.Hash{}, nil
}

func (fi *fakeImage) Manifest() (*containerregistry.Manifest, error) {
	return nil, nil
}

func (fi *fakeImage) RawManifest() ([]byte, error) {
	return []byte{}, nil
}

func (fi *fakeImage) LayerByDigest(containerregistry.Hash) (containerregistry.Layer, error) {
	return nil, nil
}

func (fi *fakeImage) LayerByDiffID(containerregistry.Hash) (containerregistry.Layer, error) {
	return nil, nil
}

type fakeIndex struct {
	manifest *containerregistry.IndexManifest
}

func (fi *fakeIndex) MediaType() (types.MediaType, error) {
	return types.DockerManifestList, nil
}

func (fi *fakeIndex) Digest() (containerregistry.Hash, error) {
	return containerregistry.Hash{
		Hex:       "abcd1234",
		Algorithm: "shafake",
	}, nil
}

func (fi *fakeIndex) Size() (int64, error) {
	return 1000, nil
}

func (fi *fakeIndex) IndexManifest() (*containerregistry.IndexManifest, error) {
	return fi.manifest, nil
}

func (fi *fakeIndex) RawManifest() ([]byte, error) {
	return []byte{}, nil
}

func (fi *fakeIndex) Image(containerregistry.Hash) (containerregistry.Image, error) {
	return nil, nil
}

func (fi *fakeIndex) ImageIndex(containerregistry.Hash) (containerregistry.ImageIndex, error) {
	return fi, nil
}

func TestNormalCase(t *testing.T) {
	remoteHelper := new(mocks.RemoteRepository)
	tarSquasher := new(mocks.TarSquasher)

	fakeIdx := &fakeIndex{
		manifest: &containerregistry.IndexManifest{
			Manifests: []containerregistry.Descriptor{
				containerregistry.Descriptor{
					Platform: &containerregistry.Platform{
						OS:           "windows",
						Architecture: "amd64",
					},
					Digest: containerregistry.Hash{
						Hex:       "abcd1999",
						Algorithm: "fakesha",
					},
				},
				containerregistry.Descriptor{
					Platform: &containerregistry.Platform{
						OS:           "linux",
						Architecture: "amd64",
					},
					Digest: containerregistry.Hash{
						Hex:       "98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4",
						Algorithm: "sha256",
					},
				},
				containerregistry.Descriptor{
					Platform: &containerregistry.Platform{
						OS:           "linux",
						Architecture: "arm64",
						Variant:      "v8",
					},
					Digest: containerregistry.Hash{
						Hex:       "abcd1999",
						Algorithm: "fakesha",
					},
				},
			},
		},
	}

	fakeImg := &fakeImage{
		layers: []containerregistry.Layer{
			&fakeLayer{
				id: "layer1",
			},
			&fakeLayer{
				id: "layer2",
			},
			&fakeLayer{
				id: "layer3",
			},
			&fakeLayer{
				id: "layer4",
			},
		},
		config: &containerregistry.ConfigFile{
			Author: "fake author",
		},
	}

	remoteHelper.On("Index", mock.MatchedBy(func(ref name.Reference) bool {
		refReal, _ := name.ParseReference("arch:latest", name.WithDefaultRegistry("index.docker.io"))
		return ref.Context() == refReal.Context() && ref.Identifier() == refReal.Identifier() && ref.Name() == refReal.Name()
	})).Return(fakeIdx, nil)

	remoteHelper.On("Image", mock.MatchedBy(func(ref name.Reference) bool {
		refReal, _ := name.ParseReference("arch@sha256:98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4", name.WithDefaultRegistry("index.docker.io"))
		return ref.Context() == refReal.Context() && ref.Identifier() == refReal.Identifier() && ref.Name() == refReal.Name()
	})).Return(fakeImg, nil)

	var extractOrder []string
	matcherLayer := func(rc io.ReadCloser) bool {
		res, _ := ioutil.ReadAll(rc)
		isLayer := strings.HasPrefix(string(res), "layerIs")
		if !isLayer {
			return false
		}
		extractOrder = append(extractOrder, string(res))
		return true
	}
	tarSquasher.On("Extract", mock.MatchedBy(matcherLayer), "squashwork").Return(nil)
	tarSquasher.On("Squash", "squashwork", "/fake/file.squash").Return(nil)

	out, cfg, err := pullAndSquashWithRemote(remoteHelper, tarSquasher, WithImage("arch", "latest"), WithOutputFile("/fake/file.squash"))

	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, out, "/fake/file.squash")
	require.Equal(t, cfg, fakeImg.config)

	require.Equal(t, extractOrder, []string{"layerIs:layer1", "layerIs:layer2", "layerIs:layer3", "layerIs:layer4"})

	remoteHelper.AssertExpectations(t)
}

func TestHandlesDifferentPlatform(t *testing.T) {
	remoteHelper := new(mocks.RemoteRepository)
	tarSquasher := new(mocks.TarSquasher)

	fakeIdx := &fakeIndex{
		manifest: &containerregistry.IndexManifest{
			Manifests: []containerregistry.Descriptor{
				containerregistry.Descriptor{
					Platform: &containerregistry.Platform{
						OS:           "linux",
						Architecture: "amd64",
					},
					Digest: containerregistry.Hash{
						Hex:       "98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4",
						Algorithm: "sha256",
					},
				},
				containerregistry.Descriptor{
					Platform: &containerregistry.Platform{
						OS:           "linux",
						Architecture: "arm64",
						Variant:      "v8",
					},
					Digest: containerregistry.Hash{
						Hex:       "98eeeeeeeeef2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4",
						Algorithm: "sha256",
					},
				},
			},
		},
	}

	fakeImg := &fakeImage{
		layers: []containerregistry.Layer{
			&fakeLayer{
				id: "layer1",
			},
			&fakeLayer{
				id: "layer2",
			},
			&fakeLayer{
				id: "layer3",
			},
			&fakeLayer{
				id: "layer4",
			},
		},
		config: &containerregistry.ConfigFile{
			Author: "fake author",
		},
	}

	remoteHelper.On("Index", mock.MatchedBy(func(ref name.Reference) bool {
		refReal, _ := name.ParseReference("arch:latest", name.WithDefaultRegistry("index.docker.io"))
		return ref.Context() == refReal.Context() && ref.Identifier() == refReal.Identifier() && ref.Name() == refReal.Name()
	})).Return(fakeIdx, nil)

	remoteHelper.On("Image", mock.MatchedBy(func(ref name.Reference) bool {
		refReal, _ := name.ParseReference("arch@sha256:98eeeeeeeeef2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4", name.WithDefaultRegistry("index.docker.io"))
		return ref.Context() == refReal.Context() && ref.Identifier() == refReal.Identifier() && ref.Name() == refReal.Name()
	})).Return(fakeImg, nil)

	var extractOrder []string
	matcherLayer := func(rc io.ReadCloser) bool {
		res, _ := ioutil.ReadAll(rc)
		isLayer := strings.HasPrefix(string(res), "layerIs")
		if !isLayer {
			return false
		}
		extractOrder = append(extractOrder, string(res))
		return true
	}
	tarSquasher.On("Extract", mock.MatchedBy(matcherLayer), "squashwork").Return(nil)
	tarSquasher.On("Squash", "squashwork", "/fake/file.squash").Return(nil)

	out, cfg, err := pullAndSquashWithRemote(remoteHelper, tarSquasher, WithImage("arch", "latest"), WithOutputFile("/fake/file.squash"), WithPlatform(platformident.PlatformAArch64))

	require.Nil(t, err)
	require.NotNil(t, cfg)
	require.Equal(t, out, "/fake/file.squash")
	require.Equal(t, cfg, fakeImg.config)

	require.Equal(t, extractOrder, []string{"layerIs:layer1", "layerIs:layer2", "layerIs:layer3", "layerIs:layer4"})

	remoteHelper.AssertExpectations(t)
}

func TestHandlesArmFallbacks(t *testing.T) {

}

func TestHandles386Fallback(t *testing.T) {

}

func TestDifferentRegistry(t *testing.T) {

}
