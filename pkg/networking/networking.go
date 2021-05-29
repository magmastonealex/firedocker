// Package networking implements the networking configurator used by firedocker.
// It allows configuring a setup in which a variety of TAP devices are joined into a bridge,
// in their own netns, with a veth pair joining that netns with the main netns.
// L2 connectivity is not provided between the main netns and VMs.
// To finish things off, you likely want IPTables rules protecting the main netns -
// ensuring that the "VM" subnet is only coming from the veth pair, not any other device.
// Internet routing/NAT may be useful to you as well...
//
// WARNING: Here be dragons. This is my first pass implementing this. There are few unit tests,
// and the overall organization of this code is a disaster.
// I'm leaving it as-is for now so that I can prove out more of the architecture, but I do want to come back
// and fix the sins in here.
//
package networking

import (
	"crypto/rand"
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

	subNetnsFd  netns.NsHandle
	mainNetnsFd netns.NsHandle

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

func getRandomMac() (net.HardwareAddr, error) {
	macBuf := make([]byte, 6)
	_, err := rand.Read(macBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to get random for MAC: %w", err)
	}

	macBuf[0] = (macBuf[0] | 2) & 0xfe // Set local bit, ensure unicast address
	return macBuf, nil
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
			return netns.None(), fmt.Errorf("netns clearing failed - could not list links %w", err)
		}

		for _, link := range links {
			// this can fail if it's a physical interface (fine)
			// or if it's a veth pair where the other side has already been deleted (fine)
			// So ignore errors.
			nlHandle.LinkDel(link)
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

func parseBridgingIps(vmSubnet string, managementSubnet string) (*bridgingNetManager, error) {
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
	var hostSideAddr net.IP
	if ones == 31 {
		hostSideAddr = manageNet.IP
	} else {
		hostSideAddr, err = getNextIP(manageNet, manageNet.IP)
		if err != nil {
			return nil, fmt.Errorf("management subnet was too small %s %w", managementSubnet, err)
		}
	}

	guestSideAddr, err := getNextIP(manageNet, hostSideAddr)
	if err != nil {
		return nil, fmt.Errorf("management subnet was too small %s %w", managementSubnet, err)
	}

	vmRouterAddr, err := getNextIP(vmNet, vmNet.IP)
	if err != nil {
		return nil, fmt.Errorf("VM subnet too small %s %w", vmSubnet, err)
	}

	return &bridgingNetManager{
		vmSubnet:     vmSubnet,
		vmRouterAddr: vmRouterAddr.String(),

		managementSubnet: managementSubnet,
		hostSideAddr:     hostSideAddr.String(),
		guestSideAddr:    guestSideAddr.String(),
	}, nil
}

func setInterfaceUpWithAddr(handle *netlink.Handle, link string, network string, ip string) error {
	ifce, err := handle.LinkByName(link)
	if err != nil {
		return fmt.Errorf("could not find link in main namespace: %w", err)
	}
	// We can now manipulate hostIfce from the main namespace.
	err = handle.LinkSetUp(ifce)
	if err != nil {
		return fmt.Errorf("could not set link up %w", err)
	}

	_, mgmtNet, err := net.ParseCIDR(network)
	if err != nil {
		return fmt.Errorf("could not parse network: %w", err)
	}
	mgmtNet.IP = net.ParseIP(ip)
	if mgmtNet.IP == nil {
		return fmt.Errorf("could not parse ip")
	}

	err = handle.AddrAdd(ifce, &netlink.Addr{IPNet: mgmtNet})
	if err != nil {
		return fmt.Errorf("could not add address to interface: %w", err)
	}
	return nil
}

func setupRoutes(bnm *bridgingNetManager) error {
	_, vmNet, err := net.ParseCIDR(bnm.vmSubnet)
	if err != nil {
		return fmt.Errorf("failed to parse vm subnet: %w", err)
	}
	_, worldNet, _ := net.ParseCIDR("0.0.0.0/0")
	guestAddr := net.ParseIP(bnm.guestSideAddr)
	if guestAddr == nil {
		return fmt.Errorf("failed to parse guest addr")
	}
	hostAddr := net.ParseIP(bnm.hostSideAddr)
	if hostAddr == nil {
		return fmt.Errorf("failed to parse host addr")
	}

	err = bnm.mainNamespace.RouteAdd(&netlink.Route{
		Dst: vmNet,
		Gw:  guestAddr,
	})
	if err != nil {
		return fmt.Errorf("Failed to add route to VM subnet: %w", err)
	}

	err = bnm.subNamespace.RouteAdd(&netlink.Route{
		Dst: worldNet,
		Gw:  hostAddr,
	})
	if err != nil {
		return fmt.Errorf("Failed to add route to VM subnet: %w", err)
	}
	return nil
}

func setupInterfaces(bnm *bridgingNetManager) error {
	vethHostMac, err := getRandomMac()
	if err != nil {
		return fmt.Errorf("could not generate mac: %w", err)
	}
	vethGuestMac, err := getRandomMac()
	if err != nil {
		return fmt.Errorf("could not generate mac: %w", err)
	}

	err = bnm.subNamespace.LinkAdd(&netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			HardwareAddr: vethHostMac,
			Name:         "fdhost0",
		},
		PeerName:         "fdguest0",
		PeerHardwareAddr: vethGuestMac,
	})
	if err != nil {
		return fmt.Errorf("could not create veth device: %w", err)
	}

	// We're going to move this device into the host namespace...
	hostIfce, err := bnm.subNamespace.LinkByName("fdhost0")
	if err != nil {
		return fmt.Errorf("could not find created ifce: %w", err)
	}
	bnm.subNamespace.LinkSetNsFd(hostIfce, int(bnm.mainNetnsFd))

	// ... and then try to fetch it on the other side!
	err = setInterfaceUpWithAddr(bnm.mainNamespace, "fdhost0", bnm.managementSubnet, bnm.hostSideAddr)
	if err != nil {
		return fmt.Errorf("could not set host interface address: %w", err)
	}
	err = setInterfaceUpWithAddr(bnm.subNamespace, "fdguest0", bnm.managementSubnet, bnm.guestSideAddr)
	if err != nil {
		return fmt.Errorf("could not set guest interface address: %w", err)
	}

	// Create our main vm bridge.
	bridgeMac, err := getRandomMac()
	if err != nil {
		return fmt.Errorf("could not get MAC for bridge: %w", err)
	}

	err = bnm.subNamespace.LinkAdd(&netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			HardwareAddr: bridgeMac,
			Name:         "vmbridge",
		},
	})
	if err != nil {
		return fmt.Errorf("could not create bridge device: %w", err)
	}

	err = setInterfaceUpWithAddr(bnm.subNamespace, "vmbridge", bnm.vmSubnet, bnm.vmRouterAddr)
	if err != nil {
		return fmt.Errorf("could not set bridge address: %w", err)
	}

	return nil
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

	// Initialization of this is _complicated_!
	// I don't really like the flow here, and testing it is a huge pain.
	// I need to figure out a better way to handle this.
	bnm, err := parseBridgingIps(vmSubnet, managementSubnet)
	if err != nil {
		return nil, fmt.Errorf("could not parse IPs: %w", err)
	}

	currentNsHandle, err := netns.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get current network namespace: %w", err)
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

	bnm.mainNetnsFd = currentNsHandle
	bnm.subNetnsFd = nsHandle
	bnm.mainNamespace = thisNlHandle
	bnm.subNamespace = guestNlHandle

	err = setupInterfaces(bnm)
	if err != nil {
		return nil, fmt.Errorf("failed to setup interfaces: %w", err)
	}

	err = setupRoutes(bnm)
	if err != nil {
		return nil, fmt.Errorf("failed to setup interfaces: %w", err)
	}

	return bnm, nil
}

// TAPInterface describes a TAP device, as well as it's MAC & IP assignment
type TAPInterface interface {
	Name() string
	Idx() int
	MAC() string
	IP() net.IP
	Netmask() net.IPMask
}
