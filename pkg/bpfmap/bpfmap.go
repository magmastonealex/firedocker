// Package bpfmap provides low-level access to BPF maps.
// I would have liked to use Cilium or Dropbox's eBPF library, but older kernels don't support the BPF command they use to figure out map information at runtime.
// This library uses `fdinfo` from procfs to determint the dimensions of the map.
// Maps are presently limited to 32 bit keys, 64 bit values for the sake of simplicity.
package bpfmap

import (
	"firedocker/pkg/bpfmap/internal"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"
)

// BPFMap represents the operations that can be performed on an open BPF map.
type BPFMap interface {
	// GetValue retrieves a value from the map.
	GetValue(key uint32) (uint64, error)
	// DeleteValue removes a value from the map. It will not return an error if the item was already deleted.
	DeleteValue(key uint32) error
	// SetValue will insert or update a value in the map.
	SetValue(key uint32, value uint64) error
	// GetCurrentValues will produce a map of keys to values representing the current state of the Map.
	GetCurrentValues() (map[uint32]uint64, error)
}

// BPFMap is a simplified type of BPF Map, specialized for this use case.
// All keys are 32bit ints, all values are 64 bit ints, all maps are of type HASH_MAP.
// If you needed to, you could extend it to include other BPF maps, but since this is all I need,
// it's all I'm bothering to implement....
type bpfMap struct {
	fd *internal.FD
}

// GetValue returns the current value for a given key, or a non-nil error if it doesn't exist.
// Be wary of concurrency with the BPF program. You can both access the space at the same time
// safely, but the value may not be what you expect...
func (mp *bpfMap) GetValue(key uint32) (uint64, error) {
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
func (mp *bpfMap) SetValue(key uint32, value uint64) error {
	// the sizes of these types are important, and I duplicated them here to drive that home.
	var valueStore uint64 = value
	var keyStore uint32 = key

	err := internal.BPFMapUpdateElem(mp.fd, internal.NewPointer(unsafe.Pointer(&keyStore)), internal.NewPointer(unsafe.Pointer(&valueStore)), internal.BPF_ANY)
	if err != nil {
		return err
	}

	return nil
}

// DeleteValue removes an element from the map.
func (mp *bpfMap) DeleteValue(key uint32) error {
	var keyStore uint32 = key

	err := internal.BPFMapDeleteElem(mp.fd, internal.NewPointer(unsafe.Pointer(&keyStore)))
	if err != nil && err != unix.ENOENT {
		return err
	}

	return nil
}

// GetCurrentValues will produce a map of keys to values representing the current state of the Map.
// Be wary of concurrency with the BPF program. You can both access the space at the same time
// safely, but the value may not be what you expect...
func (mp *bpfMap) GetCurrentValues() (map[uint32]uint64, error) {

	outMap := make(map[uint32]uint64)

	var key uint32 = 0
	var nextKey uint32 = 0

	err := internal.BPFMapGetNextKey(mp.fd, internal.NewPointer(unsafe.Pointer(&key)), internal.NewPointer(unsafe.Pointer(&nextKey)))
	key = nextKey

	for err == nil {
		var val uint64 = 0
		// this above can occasionally error due to concurrency nonsense.... you got the next valid key,
		// that's okay, this algorithm will still recover from that condition.
		if err := internal.BPFMapLookupElem(mp.fd, internal.NewPointer(unsafe.Pointer(&key)), internal.NewPointer(unsafe.Pointer(&val))); err == nil {
			outMap[key] = val
		}

		err = internal.BPFMapGetNextKey(mp.fd, internal.NewPointer(unsafe.Pointer(&key)), internal.NewPointer(unsafe.Pointer(&nextKey)))
		key = nextKey
	}
	if err != unix.ENOENT {
		return nil, err
	}

	return outMap, nil
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
func OpenMap(pinName string) (BPFMap, error) {
	fd, err := internal.BPFObjGet(pinName, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to open map FD: %w", err)
	}

	err = validateMapSizes(fd)
	if err != nil {
		return nil, fmt.Errorf("map exists but is of wrong dimensions: %w", err)
	}

	return &bpfMap{
		fd: fd,
	}, nil
}
