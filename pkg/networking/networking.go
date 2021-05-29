// Package networking implements the networking configurator used by firedocker.
// It allows configuring a setup in which a variety of TAP devices are joined into a bridge,
// in their own netns, with a veth pair joining that netns with the main netns.
// L2 connectivity is not provided between the main netns and VMs.
// To finish things off, you likely want IPTables rules protecting the main netns -
// ensuring that the "VM" subnet is only coming from the veth pair, not any other device.
// Internet routing/NAT may be useful to you as well...
package networking

import (
	"encoding/binary"
	"fmt"
	"net"
	"os/exec"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// NetworkManager represents something that can handle creating an isolated network for VMs to live on.
// Managers are initialized, and then asked to create tap devices.
// The netns name can be retrieved to allow processes that need direct access to the netns to jump in there.
// NetworkManager will by default handle IP assignment by itself - at initialization time just provide it a small (or large) subnet.
// All VMs will be assigned IPs in that range.
// (sorry in advance if the type name triggers flashbacks. I promise this NetworkManager actually does what you want it to do.)
type NetworkManager interface {
	GetVMNetns() string
	ReleaseTap(ifce TAPInterface) error
	CreateTap() (TAPInterface, error)
}

const netNSName string = "firedockerguests"

type bridgingNetManager struct {
	mainNamespace *netlink.Handle
	subNamespace  *netlink.Handle

	vmSubnet     string
	vmRouterAddr string

	managementSubnet string
	hostSideAddr     string
	guestSideAddr    string
}

// InitializeNetworkManager creates a NetworkManager
type InitializeNetworkManager func(vmSubnet string, managementSubnet string) (NetworkManager, error)

func getNextIP(network *net.IPNet, current net.IP) (net.IP, error) {
	mask := binary.BigEndian.Uint32(network.Mask)
	if current.To4() == nil || network.IP.To4() == nil {
		return nil, fmt.Errorf("only ipv4 addresses supported")
	}
	if !network.Contains(current) {
		return nil, fmt.Errorf("current must be part of network")
	}
	start := binary.BigEndian.Uint32(current.To4())
	// find the final address
	finish := (start & mask) | (mask ^ 0xffffffff)

	// a /31 is a special case - no broadcast address.
	if netSize, _ := network.Mask.Size(); netSize == 31 {
		if start >= finish {
			return nil, fmt.Errorf("subnet is full")
		}
	} else {
		// leave room for broadcast address, which is not a valid host
		if start >= finish-1 {
			return nil, fmt.Errorf("subnet is full")
		}
	}

	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, start+1)
	return ip, nil
}

func getNetNs() (netns.NsHandle, error) {
	// First try to get a reference to the netns.
	// If we _succeed_, then we need to clear everything out of it. There was some stale state left behind.
	// If there's another instance of the manager running somehow, this will be a _bad thing_.
	// We're going to assume that all devices inside the netns were created by this tool and can be destroyed.
	// Probably an assumption that will come back to bite me at some point, but at the moment I don't care :)
	handle, err := netns.GetFromName(netNSName)
	if err == nil {
		// Oops. Clean up whatever the heck is in here....
		nlHandle, err := netlink.NewHandleAt(handle)
		if err != nil {
			return netns.None(), fmt.Errorf("netns clearing failed - could not open handle %w", err)
		}
		// Deleting the veth inside the netns will also remove the one attached to the host.
		links, err := nlHandle.LinkList()
		if err != nil {
			return netns.None(), fmt.Errorf("netns clearing failed - could not list links %w", links)
		}

		for _, link := range links {
			err := nlHandle.LinkDel(link)
			if err != nil {
				return netns.None(), fmt.Errorf("netns clearing failed - could not delete link %s %w", link.Attrs().Name, err)
			}
		}

		nlHandle.Delete()

		delCmd := exec.Command("ip", "netns", "del", netNSName)
		err = delCmd.Run()
		if err != nil {
			return netns.None(), fmt.Errorf("failed to delete old netns: %w", err)
		}
	}

	addCmd := exec.Command("ip", "netns", "add", netNSName)
	err = addCmd.Run()
	if err != nil {
		return netns.None(), fmt.Errorf("failed to add new netns: %w", err)
	}

	handle, err = netns.GetFromName(netNSName)
	if err != nil {
		return netns.None(), fmt.Errorf("failed to retrieve netns after creation. This is very odd %w", err)
	}
	return handle, nil
}

// InitializeBridgingNetworkManager will initialize a NetworkManager
// which creates a bridge in a secondary network namespace.
// vmSubnet is used to provide addresses to VMs.
// managementSubnet is used to provide a route to the VM subnet. It should probably be a /31.
// A route will be set up for `vmSubnet` via the management veth pair.
// You should not be attempting to share IP space. The manager will have complete control over these
// two subnets.
// This is a very opinionated NetworkManager. It's possible you have a need to perform all this initialization work
// _outside_ of firedocker. Another type of NetworkManager may be a better fit.
func InitializeBridgingNetworkManager(vmSubnet string, managementSubnet string) (NetworkManager, error) {
	_, vmNet, err := net.ParseCIDR(vmSubnet)
	if err != nil {
		return nil, fmt.Errorf("bad VM subnet %s %w", vmSubnet, err)
	}
	ones, bits := vmNet.Mask.Size()
	if bits != 32 || vmNet.IP.To4() == nil {
		return nil, fmt.Errorf("VM subnet must be ipv4 %s", vmSubnet)
	}
	if ones > 31 {
		return nil, fmt.Errorf("VM subnet must contain room for at least two hosts %s", vmSubnet)
	}

	_, manageNet, err := net.ParseCIDR(managementSubnet)
	if err != nil {
		return nil, fmt.Errorf("bad management subnet %s %w", managementSubnet, err)
	}
	ones, bits = manageNet.Mask.Size()
	if bits != 32 || manageNet.IP.To4() == nil {
		return nil, fmt.Errorf("management subnet must be ipv4 %s", managementSubnet)
	}
	if ones > 31 {
		return nil, fmt.Errorf("management subnet must contain room for at least two hosts %s", managementSubnet)
	}

	// We know we've got valid networks. Pull out our addresses.
	hostSideAddr, err := getNextIP(manageNet, manageNet.IP)
	if err != nil {
		return nil, fmt.Errorf("management subnet was too small %s %w", managementSubnet, err)
	}
	guestSideAddr, err := getNextIP(manageNet, hostSideAddr)
	if err != nil {
		return nil, fmt.Errorf("management subnet was too small %s %w", managementSubnet, err)
	}

	vmRouterAddr, err := getNextIP(vmNet, vmNet.IP)
	if err != nil {
		return nil, fmt.Errorf("VM subnet too small %s %w", vmSubnet, err)
	}

	nsHandle, err := getNetNs()
	if err != nil {
		return nil, fmt.Errorf("failed to create network namespace: %w", err)
	}

	guestNlHandle, err := netlink.NewHandleAt(nsHandle)
	if err != nil {
		return nil, fmt.Errorf("could not open handle in guest netns: %w", err)
	}

	thisNlHandle, err := netlink.NewHandle()
	if err != nil {
		return nil, fmt.Errorf("could not open handle for current netns: %w", err)
	}

	return &bridgingNetManager{
		mainNamespace: thisNlHandle,
		subNamespace:  guestNlHandle,

		vmSubnet:     vmSubnet,
		vmRouterAddr: vmRouterAddr.String(),

		managementSubnet: managementSubnet,
		hostSideAddr:     hostSideAddr.String(),
		guestSideAddr:    guestSideAddr.String(),
	}, nil
}

// TAPInterface describes a TAP device, as well as it's MAC & IP assignment
type TAPInterface interface {
	Name() string
	Idx() int
	MAC() string
	IP() net.IP
	Netmask() net.IPMask
}
