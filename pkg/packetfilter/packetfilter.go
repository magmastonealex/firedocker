// Package packetfilter provides capabilities of filtering traffic on a particular interface
// to ensure it only ingresses traffic from specific IPs or MACs.
// This is particularly helpful with Firecracker VMs, because it allows filtering the TAP interface
// exposed to the VM to prevent it from spoofing packets or pretending to have a different IP or MAC
// from one it was assigned.
// Filtering is implemented using TC eBPF. The eBPF program expects a map defined for allowed IPs and allowed MACs per ifindex.
// Helper functions are provided to install the eBPF filter, remove the eBPF filter, and add and remove entries
// for IP and MAC whitelisting.
package packetfilter

import (
	"firedocker/pkg/bpfmap"

	"github.com/vishvananda/netlink"
)

// PacketWhitelister sets up the eBPF filtering on a given interface to permit only a single IP and MAC
// to be ingressed on that interface.
// Currently, you can only install on interfaces in the same network-namespace as the manager.
// You probably want to create a tuntap device, install the filter, and then move the device into it's destination namespace.
// You may update the device after moving to accept a different IP/MAC assuming you kept track of it's interface index.
type PacketWhitelister interface {
	// Install will set up whitelisting on the provided interface.
	Install(idx uint32, ip string, mac string) error
	// UpdateByIndex will update the whitelist for a particular interface index
	UpdateByIndex(idx uint32, ip string, mac string) error
}

type netlinkHelper interface {
	LinkByIndex(index int) (netlink.Link, error)
}

type tcHelper interface {
	// Ensure that a queueing discipline (qdisc) of type clsact is assigned to the specified interface.
	EnsureQdiscClsact(ifce string) error
	LoadBPFIngress(ifce string, path string) error
}

// DefaultPacketWhitelister implements packet whitelisting using TC & eBPF.
type DefaultPacketWhitelister struct {
	nlHelper  netlinkHelper
	tcHelper  tcHelper
	bpfOpener func(pinName string) (bpfmap.BPFMap, error)
}

// helper function to initialize a netlink handle if one is not already set up.
func (dp *DefaultPacketWhitelister) ensureNetlink() error {
	if dp.nlHelper != nil {
		return nil
	}
	handle, err := netlink.NewHandle()
	if err != nil {
		return err
	}
	dp.nlHelper = handle
	return nil
}

// Initialize with default netlink, TC, and BPF helpers.
func (dp *DefaultPacketWhitelister) initialize() error {
	if err := dp.ensureNetlink(); err != nil {
		return err
	}
	if dp.tcHelper == nil {
		dp.tcHelper = &tcHelperImpl{}
	}
	if dp.bpfOpener == nil {
		dp.bpfOpener = bpfmap.OpenMap
	}

	return nil
}

// Install implements PacketWhitelister.Install
func (dp *DefaultPacketWhitelister) Install(idx uint32, ip string, mac string) error {
	if err := dp.initialize(); err != nil {
		return err
	}

	return nil
}

// UpdateByIndex implements PacketWhitelister.UpdateByIndex
func (dp *DefaultPacketWhitelister) UpdateByIndex(idx uint32, ip string, mac string) error {
	return nil
}
