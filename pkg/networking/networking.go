// Package networking implements the networking configurator used by firedocker.
// It allows configuring a setup in which a variety of TAP devices are joined into a bridge
// Internet routing/NAT may be useful to you as well...
//
// WARNING: Here be dragons. This is my first pass implementing this. There are few unit tests,
// and the overall organization of this code is a disaster.
// I'm leaving it as-is for now so that I can prove out more of the architecture, but I do want to come back
// and fix the sins in here.
//
// TODO: This is truly proof-of-concept. In reality, the VM interfaces should be created in a separate netns,
// with just one veth pair to connect the two. Routing rules can be put in place to bridge things up properly.
package networking

import (
	"crypto/rand"
	"encoding/binary"
	"firedocker/pkg/packetfilter"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// NetworkManager represents something that can handle creating an isolated network for VMs to live on.
// Managers are initialized, and then asked to create tap devices.
// NetworkManager will by default handle IP assignment by itself - at initialization time just provide it a small (or large) subnet.
// All VMs will be assigned IPs in that range.
// (sorry in advance if the type name triggers flashbacks. I promise this NetworkManager actually does what you want it to do.)
type NetworkManager interface {
	ReleaseTap(ifce TAPInterface) error
	CreateTap() (TAPInterface, error)
}

// TAPInterface describes a TAP device, as well as it's MAC & IP assignment
type TAPInterface interface {
	Name() string
	Idx() int
	MAC() string
	IP() net.IP
	Netmask() net.IPMask
}

type bridgingNetManager struct {
	mainNamespace *netlink.Handle

	packetFilter packetfilter.PacketWhitelister

	subNetnsFd  netns.NsHandle
	mainNetnsFd netns.NsHandle

	bridgeLinkIdx int

	vmSubnet       *net.IPNet
	vmRouterAddr   net.IP
	vmLastAssigned net.IP
}

// InitializeNetworkManager creates a NetworkManager
type InitializeNetworkManager func(vmSubnet string) (NetworkManager, error)

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

func parseBridgingIps(vmSubnet string) (*bridgingNetManager, error) {
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

	// We know we've got valid networks. Pull out our addresses.
	vmRouterAddr, err := getNextIP(vmNet, vmNet.IP)
	if err != nil {
		return nil, fmt.Errorf("VM subnet too small %s %w", vmSubnet, err)
	}

	return &bridgingNetManager{
		vmSubnet:       vmNet,
		vmRouterAddr:   vmRouterAddr,
		vmLastAssigned: vmRouterAddr,
	}, nil
}

func setInterfaceUpWithAddr(handle *netlink.Handle, ifce netlink.Link, network *net.IPNet, ip net.IP) error {
	// We can now manipulate hostIfce from the main namespace.
	err := handle.LinkSetUp(ifce)
	if err != nil {
		return fmt.Errorf("could not set link up %w", err)
	}

	err = handle.AddrAdd(ifce, &netlink.Addr{IPNet: &net.IPNet{
		IP:   ip,
		Mask: network.Mask,
	}})
	if err != nil {
		return fmt.Errorf("could not add address to interface: %w", err)
	}
	return nil
}

func setupInterfaces(bnm *bridgingNetManager) error {
	// if the bridge exists at startup, delete it
	// TODO: we have more cleanup to do
	if link, err := bnm.mainNamespace.LinkByName("vmbridge"); err == nil {
		fmt.Println("Cleaning up old bridge")
		allLinks, err := bnm.mainNamespace.LinkList()
		if err != nil {
			return fmt.Errorf("failed to list links: %w", err)
		}
		for _, lnk := range allLinks {
			if lnk.Attrs().MasterIndex == link.Attrs().Index {
				fmt.Printf("deleting %s\n", lnk.Attrs().Name)
				bnm.mainNamespace.LinkDel(lnk)
			}
		}
		bnm.mainNamespace.LinkDel(link)
	}

	// Create our main vm bridge.
	bridgeMac, err := getRandomMac()
	if err != nil {
		return fmt.Errorf("could not get MAC for bridge: %w", err)
	}

	bridgeIfce := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			HardwareAddr: bridgeMac,
			Name:         "vmbridge",
		},
	}
	err = bnm.mainNamespace.LinkAdd(bridgeIfce) // fills out bridgeIfce idx, etc.
	if err != nil {
		return fmt.Errorf("could not create bridge device: %w", err)
	}

	err = setInterfaceUpWithAddr(bnm.mainNamespace, bridgeIfce, bnm.vmSubnet, bnm.vmRouterAddr)
	if err != nil {
		return fmt.Errorf("could not set bridge address: %w", err)
	}

	bnm.bridgeLinkIdx = bridgeIfce.Attrs().Index

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
func InitializeBridgingNetworkManager(vmSubnet string) (NetworkManager, error) {

	// Initialization of this is _complicated_!
	// I don't really like the flow here, and testing it is a huge pain.
	// I need to figure out a better way to handle this.
	bnm, err := parseBridgingIps(vmSubnet)
	if err != nil {
		return nil, fmt.Errorf("could not parse IPs: %w", err)
	}

	currentNsHandle, err := netns.Get()
	if err != nil {
		return nil, fmt.Errorf("failed to get current network namespace: %w", err)
	}

	thisNlHandle, err := netlink.NewHandle()
	if err != nil {
		return nil, fmt.Errorf("could not open handle for current netns: %w", err)
	}

	bnm.mainNetnsFd = currentNsHandle
	bnm.mainNamespace = thisNlHandle

	err = setupInterfaces(bnm)
	if err != nil {
		return nil, fmt.Errorf("failed to setup interfaces: %w", err)
	}

	bnm.packetFilter = &packetfilter.DefaultPacketWhitelister{}

	return bnm, nil
}
