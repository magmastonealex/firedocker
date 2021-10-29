// Package firecracker is used to start with/interact with
// a running firecracker VMM.
package firecracker

import (
	"bytes"
	"context"
	"encoding/json"
	"firedocker/pkg/networking"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/google/uuid"
)

type ContainerRuntimeConfig struct {
	Entrypoint  []string
	Cmd         []string
	Environment []string
	Workdir     string
}

type Config struct {
	RootFilesystemPath    string
	ScratchFilesystemPath string // TODO: Create a "StorageManager" which can handle this.
	// TODO: we also need a "config" filesystem which the container can persist data across versions in.
	// The existing "scratch" filesystem is linked with the root fs (but allows two containers based on same rootfs)
	NetworkInterface networking.TAPInterface

	RuntimeConfig ContainerRuntimeConfig
}

type VMInstance interface {
	ID() string
	Shutdown()
	ConfigureAndStart(Config) error
	Wait()
}

type Manager interface {
	StartInstance() (VMInstance, error)
}

// TODO: VMInstance ought to act as a watchdog for comms with the init application.
type vmInstance struct {
	id       string
	sockpath string

	started  bool
	finished bool
	closed   chan struct{}

	listenSock net.Listener

	proc *os.Process
}

// TODO: manager should track all the running instances and support global shutdown, etc.
type manager struct{}

func CreateManager() Manager {
	return &manager{}
}

// TODO: this should launch firecracker via the jailer, rather than directly.
func (m *manager) StartInstance() (VMInstance, error) {
	vmId := uuid.NewString()
	instance := &vmInstance{
		id:       vmId,
		sockpath: fmt.Sprintf("/run/firedocker/%s/vm.sock", vmId[:10]),
		closed:   make(chan struct{}, 1),
	}
	os.MkdirAll(fmt.Sprintf("/run/firedocker/%s", vmId[:10]), 0o770)

	cmd := exec.Command("./firecracker", "--id", instance.id, "--api-sock", instance.sockpath)
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

	err = cmd.Start()
	if err != nil {
		return nil, fmt.Errorf("failed to start command", err)
	}
	instance.proc = cmd.Process
	go instance.wait()

	return instance, nil
}

func (vmi *vmInstance) wait() {
	vmi.proc.Wait()
	vmi.proc = nil
	vmi.finished = true
	close(vmi.closed)
	log.Println("VM instance exited")
	// TODO: track this and emit events when things have stopped working.
}

func (vmi *vmInstance) Wait() {
	<-vmi.closed
}

func (vmi *vmInstance) ID() string {
	return vmi.id
}

func (vmi *vmInstance) Shutdown() {
	// TODO: signal via the init process to gracefully shut down
	// before killing it.
	if vmi.proc != nil {
		vmi.proc.Kill()
	}
	vmi.Wait()
}

func (vmi *vmInstance) do(req *http.Request) (*http.Response, error) {
	client := &http.Client{
		Timeout: time.Second * 5,
		Transport: &http.Transport{
			DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
				return net.Dial("unix", vmi.sockpath)
			},
		},
	}

	return client.Do(req)
}

func (vmi *vmInstance) waitForOnline() error {
	// try to connect to Firecracker for 2 seconds.
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2*time.Second))
	defer cancel()
	for {
		time.Sleep(10 * time.Millisecond)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		req, _ := http.NewRequest(http.MethodGet, "http://localhost/", nil)
		resp, err := vmi.do(req)
		if err != nil {
			// expected, for a while. Thus the timeout...
			fmt.Printf("failed connecting to FC: %+v", err)
			continue
		}
		if resp.StatusCode == 200 {
			return nil
		}
	}
}

func (vmi *vmInstance) doPut(path string, body interface{}) error {
	jsonBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("could not serialize body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("http://localhost%s", path), bytes.NewBuffer(jsonBytes))
	if err != nil {
		return fmt.Errorf("failed to assemble request: %w", err)
	}

	resp, err := vmi.do(req)
	if err != nil {
		return fmt.Errorf("failed to retrieve: %+w", err)
	}

	if resp.StatusCode != 204 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("non-204 response, and failed to read body: %+w", err)
		}
		resp.Body.Close()
		return fmt.Errorf("non-204 response: %s", string(bodyBytes))
	}
	return nil
}

func (vmi *vmInstance) ConfigureAndStart(config Config) error {
	if vmi.started {
		return fmt.Errorf("vm already started")
	}

	if err := vmi.waitForOnline(); err != nil {
		return err
	}

	// Set machine config...
	if err := vmi.doPut("/machine-config", &machineConfiguration{
		NumVCPUs:      1,
		MemorySizeMiB: 256,
	}); err != nil {
		return fmt.Errorf("failed to set machine config: %+w", err)
	}
	// Set up kernel...
	if err := vmi.doPut("/boot-source", &bootSource{
		KernelImg:  "./vmlinux",
		InitRDPath: "./initrd.cpio",
		BootArgs:   "console=ttyS0 reboot=k panic=1 pci=off",
	}); err != nil {
		return fmt.Errorf("failed to set boot config: %+w", err)
	}
	// Set up drives...
	if err := vmi.doPut("/drives/vda", &drive{
		DriveID:  "vda",
		ReadOnly: true,
		Path:     config.RootFilesystemPath,
	}); err != nil {
		return fmt.Errorf("failed to set root drive: %+w", err)
	}

	if err := vmi.doPut("/drives/vdb", &drive{
		DriveID:  "vdb",
		ReadOnly: false,
		Path:     config.ScratchFilesystemPath,
	}); err != nil {
		return fmt.Errorf("failed to set scratch drive: %+w", err)
	}

	// Set up network interfaces
	if err := vmi.doPut("/network-interfaces/eth0", &networkInterface{
		AllowMMDS:     true,
		IfceID:        "eth0",
		HostInterface: config.NetworkInterface.Name(),
		GuestMAC:      config.NetworkInterface.MAC(),
	}); err != nil {
		return fmt.Errorf("failed to set boot config: %+w", err)
	}

	// Set MMDS settings
	//figure out CIDR representation:
	netmaskOnes, _ := config.NetworkInterface.Netmask().Size()

	serializedNetwork, err := json.Marshal(&mmdsIPConfig{
		IPCIDR:       fmt.Sprintf("%s/%d", config.NetworkInterface.IP().String(), netmaskOnes),
		PrimaryDNS:   "8.8.8.8",
		SecondaryDNS: "8.8.4.4",
		Routes: []mmdsRoute{{
			Gw:      config.NetworkInterface.DefaultGateway().String(),
			Network: "0.0.0.0/0",
		}},
	})
	if err != nil {
		return fmt.Errorf("failed to serialize network configuration: %w", err)
	}

	serializedRuntimeConfig, err := json.Marshal(&config.RuntimeConfig)
	if err != nil {
		return fmt.Errorf("failed to serialize runtime configuration: %w", err)
	}

	if err := vmi.doPut("/mmds", &mmdsInfo{
		IPConfig:      string(serializedNetwork),
		RuntimeConfig: string(serializedRuntimeConfig),
	}); err != nil {
		return fmt.Errorf("failed to set MMDS config: %+w", err)
	}

	// Start instance
	if err := vmi.doPut("/actions", &action{
		Type: "InstanceStart",
	}); err != nil {
		return fmt.Errorf("failed to start VM: %+w", err)
	}
	vmi.started = true
	return nil
}
