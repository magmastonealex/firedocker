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
	"io/ioutil"
	"os"
	"strings"
	"unsafe"
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

	// the sizes of these types are of critical importance. Do not change them without reason.
	// (that sounded worse than it is. The kernel function will take a *uint64_t and *uint32_t, and we can't safely
	//  construct pointers that can cross the kernel boundary _and_ that respect Go's type system. This function,
	//  and others in this package, are the boundary line between "safe" and "unsafe" code.)
	var value uint64
	var keyStore uint32 = key

	err := internal.BPFMapLookupElem(mp.fd, internal.NewPointer(unsafe.Pointer(&keyStore)), internal.NewPointer(unsafe.Pointer(&value)))
	if err != nil {
		// usually ENOENT.
		return 0, err
	}

	return value, nil
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
// There's lots of good stuff in this file, we just don't currently need much of it.
func validateMapSizes(fd *internal.FD) error {
	fdVal, err := fd.Value()
	if err != nil {
		return fmt.Errorf("can't get raw fd value: %w", err)
	}
	contents, err := ioutil.ReadFile(fmt.Sprintf("/proc/%d/fdinfo/%d", os.Getpid(), fdVal))
	if err != nil {
		return fmt.Errorf("failed to read contents of fdinfo")
	}

	kvs := strings.Split(string(contents), "\n")
	var mapType int = -1
	var mapValSize int = -1
	var mapKeySize int = -1
	for _, kv := range kvs {
		var val int = -1
		if read, _ := fmt.Sscanf(kv, "map_type:\t%d", &val); read == 1 {
			mapType = val
		} else if read, _ := fmt.Sscanf(kv, "key_size:\t%d", &val); read == 1 {
			mapKeySize = val
		} else if read, _ := fmt.Sscanf(kv, "value_size:\t%d", &val); read == 1 {
			mapValSize = val
		}
	}
	if mapType == -1 || mapValSize == -1 || mapKeySize == -1 {
		return fmt.Errorf("failed to read type, valSize, keySize. something is wrong with this map")
	}

	if mapType != 1 {
		return fmt.Errorf("currently only hashmap-type maps are supported")
	}

	if mapValSize != 8 {
		return fmt.Errorf("currently all values must be 8 bytes")
	}

	if mapKeySize != 4 {
		return fmt.Errorf("currently all keys must be 4 bytes")
	}

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
