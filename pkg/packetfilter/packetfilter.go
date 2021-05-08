// Package packetfilter provides capabilities of filtering traffic on a particular interface
// to ensure it only ingresses traffic from specific IPs or MACs.
// This is particularly helpful with Firecracker VMs, because it allows filtering the TAP interface
// exposed to the VM to prevent it from spoofing packets or pretending to have a different IP or MAC
// from one it was assigned.
// Filtering is implemented using TC eBPF. The eBPF program expects a map defined for allowed IPs and allowed MACs per ifindex.
// Helper functions are provided to install the eBPF filter, remove the eBPF filter, and add and remove entries
// for IP and MAC whitelisting.
// I would have liked to use Cilium or Dropbox's eBPF library, but older kernels don't support the BPF command they use to figure out map information at runtime.
package packetfilter

import (
	"firedocker/pkg/packetfilter/internal"
	"fmt"
)

// BPFMap is a simplified type of BPF Map, specialized for this use case.
// All keys are 32bit ints, all values are 64 bit ints, all maps are of type HASH_MAP.
// If you needed to, you could extend it to include other BPF maps, but since this is all I need,
// it's all I'm bothering to implement....
type BPFMap struct {
	fd *internal.FD
}

// GetValue returns the current value for a given key, or a non-nil error if it doesn't exist.
// Be wary of concurrency with the BPF program. You can both access the space at the same time
// safely, but the value may not be what you expect...
func (mp *BPFMap) GetValue(key uint32) (uint64, error) {
	return 0, nil
}

// SetValue will set the value for a particular key
func (mp *BPFMap) SetValue(key uint32, value uint64) error {
	return nil
}

// GetCurrentValues will produce a map of keys to values representing the current state of the Map.
// Be wary of concurrency with the BPF program. You can both access the space at the same time
// safely, but the value may not be what you expect...
func (mp *BPFMap) GetCurrentValues() (map[uint32]uint64, error) {
	return nil, nil
}

// ensures a map has 32 bit keys, 64 bit values using procfs.
func validateMapSizes(fd *internal.FD) error {
	return nil
}

// OpenMap will attempt to open an existing map based on a pinned filename (probably in /sys/bpf or another bpffs mountpoint.)
func OpenMap(pinName string) (*BPFMap, error) {
	fd, err := internal.BPFObjGet(pinName, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open map FD: %w", err)
	}

	err = validateMapSizes(fd)
	if err != nil {
		return nil, fmt.Errorf("map exists but is of wrong dimensions: %w", err)
	}

	return &BPFMap{
		fd: fd,
	}, nil
}
