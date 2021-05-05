package main

import (
	"firedocker/cmd/preinit/mocks"
	"os"
	"testing"
)

// This looks like a reverse implementation.
// It's based on system-manager docs, as well as a few resources on having an overlay rootfs.
// The intention of this test is to prove everything that needs calling is being called.
func TestMountsProperly(t *testing.T) {
	mh := new(mocks.MountHelperMock)
	mh.On("MustMkdir", "/ro", os.FileMode(0755))
	mh.On("MustMkdir", "/rw", os.FileMode(0755))
	mh.On("MustMkdir", "/realroot", os.FileMode(0755))

	mountAndPivotWithHelper(mh)

	mh.AssertExpectations(t)
}
