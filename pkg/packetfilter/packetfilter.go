// Package packetfilter provides capabilities of filtering traffic on a particular interface
// to ensure it only ingresses traffic from specific IPs or MACs.
// This is particularly helpful with Firecracker VMs, because it allows filtering the TAP interface
// exposed to the VM to prevent it from spoofing packets or pretending to have a different IP or MAC
// from one it was assigned.
// Filtering is implemented using TC eBPF. The eBPF program expects a map defined for allowed IPs and allowed MACs per ifindex.
// Helper functions are provided to install the eBPF filter, remove the eBPF filter, and add and remove entries
// for IP and MAC whitelisting.
// WARNING: This is not a replacement for a firewall. It's intended to deal with malicious behavior that can happen below
// where something like iptables can handle it. All it does is ensure all packets coming FROM a VM:
//   - are from the MAC assigned to the VM
//   - have a source IP of the VM
//   - Are IPv4 or ARP
//   - ARP packets coming from the VM aren't attempting to poison caches or otherwise cause malaise.
package packetfilter

import (
	"bufio"
	"firedocker/pkg/bpfmap"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/vishvananda/netlink"
)

// PacketWhitelister sets up the eBPF filtering on a given interface to permit only a single IP and MAC
// to be ingressed on that interface.
// Currently, you can only install on interfaces in the same network-namespace as the manager.
// You probably want to create a tuntap device, install the filter, and then move the device into it's destination namespace.
// You may update the device after moving to accept a different IP/MAC assuming you kept track of it's interface index.
type PacketWhitelister interface {
	// Install will set up whitelisting on the provided interface.
	Install(idx int, ip string, mac string) error
	// UpdateByIndex will update the whitelist for a particular interface index
	UpdateByIndex(idx int, ip string, mac string) error
}

type netlinkHelper interface {
	LinkByIndex(index int) (netlink.Link, error)
}

type tcHelper interface {
	// Ensure that a queueing discipline (qdisc) of type clsact is assigned to the specified interface.
	EnsureQdiscClsact(ifce string) error
	LoadBPFIngress(ifce string, path string) error
}

type bpfOpener func(pinName string) (bpfmap.BPFMap, error)

// DefaultPacketWhitelister implements packet whitelisting using TC & eBPF.
type DefaultPacketWhitelister struct {
	nlHelper  netlinkHelper
	tcHelper  tcHelper
	bpfOpener bpfOpener
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
func (dp *DefaultPacketWhitelister) Install(idx int, ip string, mac string) error {
	if err := dp.initialize(); err != nil {
		return err
	}

	lnk, err := dp.nlHelper.LinkByIndex(idx)
	if err != nil {
		return fmt.Errorf("unknown link with index %d: %w", idx, err)
	}
	linkName := lnk.Attrs().Name

	file, err := ioutil.TempFile("", "firedocker_filter")
	if err != nil {
		return fmt.Errorf("failed to create temp file %w", err)
	}
	defer os.Remove(file.Name())
	defer file.Close()

	w := bufio.NewWriter(file)
	_, err = w.Write(bpfFilterContents)
	if err != nil {
		return fmt.Errorf("failed to write filter data: %w", err)
	}
	err = w.Flush()
	if err != nil {
		return fmt.Errorf("failed to write filter data: %w", err)
	}
	file.Close()

	err = dp.tcHelper.EnsureQdiscClsact(linkName)
	if err != nil {
		return fmt.Errorf("failed to set up clsact qdisc: %w", err)
	}

	err = dp.tcHelper.LoadBPFIngress(linkName, file.Name())
	if err != nil {
		return fmt.Errorf("failed to insert filter: %w", err)
	}

	return dp.UpdateByIndex(idx, ip, mac)
}

// UpdateByIndex implements PacketWhitelister.UpdateByIndex
func (dp *DefaultPacketWhitelister) UpdateByIndex(idx int, ip string, mac string) error {
	if err := dp.initialize(); err != nil {
		return err
	}

	// start by converting IP and MAC into uint64s suitable for setting in the map.
	ipParsed := net.ParseIP(ip).To4()
	if ipParsed == nil {
		return fmt.Errorf("ip %s is not valid", ip)
	}

	macParsed, err := net.ParseMAC(mac)
	if err != nil {
		return fmt.Errorf("could not parse MAC %s. %w", mac, err)
	}
	if len(macParsed) != 6 {
		return fmt.Errorf("%s is not an Ethernet MAC", mac)
	}

	var macUint uint64 = 0 | uint64(macParsed[0])<<40 |
		uint64(macParsed[1])<<32 |
		uint64(macParsed[2])<<24 |
		uint64(macParsed[3])<<16 |
		uint64(macParsed[4])<<8 |
		uint64(macParsed[5])<<0

	var ipUint uint64 = 0 | uint64(ipParsed[3])<<24 |
		uint64(ipParsed[2])<<16 |
		uint64(ipParsed[1])<<8 |
		uint64(ipParsed[0])<<0

	ipMap, err := dp.bpfOpener("/sys/fs/bpf/tc/globals/ifce_allowed_ip")
	if err != nil {
		return fmt.Errorf("failed to open ip map: %w", err)
	}
	defer ipMap.Close()
	err = ipMap.SetValue(uint32(idx), ipUint)
	if err != nil {
		return fmt.Errorf("failed to set IP value in map: %w", err)
	}

	macMap, err := dp.bpfOpener("/sys/fs/bpf/tc/globals/ifce_allowed_macs")
	if err != nil {
		return fmt.Errorf("failed to open mac map: %w", err)
	}
	defer macMap.Close()
	err = macMap.SetValue(uint32(idx), macUint)
	if err != nil {
		return fmt.Errorf("failed to set mac in map: %w", err)
	}

	return nil
}
