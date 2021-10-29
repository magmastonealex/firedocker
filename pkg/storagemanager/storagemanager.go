// Package storagemanager is responsible for allocating & removing ext4 filesystem images.
// In the future, it should be backed by something interesting, for now, just the filesystem.
package storagemanager

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"syscall"
)

type Manager interface {
	// GetFilesystemImage will create a filesystem with a unique ID id.
	CreateFilesystemImage(id string, sizeMB int) (string, error)
}

type rawStorageManager struct {
	basePath string
}

func CreateRawStorageManager(basePath string) Manager {
	return &rawStorageManager{
		basePath: basePath,
	}
}

func (rsm *rawStorageManager) CreateFilesystemImage(id string, sizeMB int) (string, error) {
	filePath := path.Join(rsm.basePath, fmt.Sprintf("%s.ext4", id))

	f, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("could not create file %s: %w", filePath, err)
	}

	if err := syscall.Fallocate(int(f.Fd()), 0, 0, int64(sizeMB)*int64(1000000)); err != nil {
		f.Close()
		os.Remove(filePath)
		return "", fmt.Errorf("could not fallocate %s: %w", filePath, err)
	}
	f.Close()

	if err := exec.Command("mkfs.ext4", filePath).Run(); err != nil {
		os.Remove(filePath)
		return "", fmt.Errorf("failed to mkfs.ext4: %w", err)
	}

	return filePath, nil
}
