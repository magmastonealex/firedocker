package dockersquasher

import (
	"github.com/google/go-containerregistry/pkg/name"
	containerregistry "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

type remoteRepository interface {
	Index(ref name.Reference, options ...remote.Option) (containerregistry.ImageIndex, error)
	Image(ref name.Reference, options ...remote.Option) (containerregistry.Image, error)
}

type remoteRepositoryImpl struct{}

func (rri remoteRepositoryImpl) Index(ref name.Reference, options ...remote.Option) (containerregistry.ImageIndex, error) {
	return remote.Index(ref, options...)
}

func (rri remoteRepositoryImpl) Image(ref name.Reference, options ...remote.Option) (containerregistry.Image, error) {
	return remote.Image(ref, options...)
}
