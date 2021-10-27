// Package firecracker is used to start with/interact with
// a running firecracker VMM.
package firecracker

import (
	"firedocker/pkg/networking"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/google/uuid"
)

type Config struct {
	ID                    string
	RootFilesystemPath    string
	ScratchFilesystemPath string // TODO: Create a "StorageManager" which can handle this.
	NetworkInterfaces     []networking.TAPInterface
	ConfigKeys            map[string]string
}

type VMInstance interface {
	ID() string
	Shutdown() error // Shuts down the VM within 5 seconds - first gracefully then SIGKILL.
}

type Manager interface {
	StartVM(config Config) (VMInstance, error)
}

type vmInstance struct {
	id       string
	sockpath string
}

type manager struct{}

// TODO: this should launch firecracker via the jailer, rather than directly.
func (m *manager) StartVM(config Config) (VMInstance, error) {
	vmId := uuid.NewString()
	os.MkdirAll(fmt.Sprintf("/run/firedocker/%s", vmId), 0o770)
	cmd := exec.Command("./firecracker", "--id", vmId, "--api-sock", fmt.Sprintf("/run/firedocker/%s/vm.sock", vmId))
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to get stderr: %w", err)
	}

	go io.CopyBuffer(os.Stdout, stdout, make([]byte, 255))
	go io.CopyBuffer(os.Stdout, stderr, make([]byte, 255))

	cmd.Start()
}

// Start up a VM.
// Launch the Firecracker VMM
// Start configuring the VM with disks, network interfaces, etc.
// Set config keys via mmds
// Start the instance.
