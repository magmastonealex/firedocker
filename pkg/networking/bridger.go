package networking

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

type bnmTAPInterface struct {
	name    string
	idx     int
	mac     string
	ip      net.IP
	netmask net.IPMask

	dgw net.IP
}

func (bt *bnmTAPInterface) DefaultGateway() net.IP {
	return bt.dgw
}
func (bt *bnmTAPInterface) Name() string {
	return bt.name
}
func (bt *bnmTAPInterface) Idx() int {
	return bt.idx
}
func (bt *bnmTAPInterface) MAC() string {
	return bt.mac
}
func (bt *bnmTAPInterface) IP() net.IP {
	return bt.ip
}
func (bt *bnmTAPInterface) Netmask() net.IPMask {
	return bt.netmask
}

func (bnm *bridgingNetManager) ReleaseTap(ifce TAPInterface) error {
	// Try to cast it back to a bnm type.
	bnmType, ok := ifce.(*bnmTAPInterface)
	if !ok {
		return fmt.Errorf("passed TAPInterface was not from this NetworkManager")
	}

	// Use netns netlink to delete the TAP device

	link, err := bnm.mainNamespace.LinkByIndex(bnmType.idx)
	if err != nil {
		return fmt.Errorf("could not find link by idx: %w", err)
	}
	err = bnm.mainNamespace.LinkDel(link)
	if err != nil {
		return fmt.Errorf("could not delete link: %w", err)
	}
	// TODO: track IP allocations instead of just "next".
	return nil
}

func (bnm *bridgingNetManager) CreateTap() (TAPInterface, error) {
	// Create tuntap device
	mac, err := getRandomMac()
	if err != nil {
		return nil, fmt.Errorf("could not create MAC: %w", err)
	}
	ipAddr, err := getNextIP(bnm.vmSubnet, bnm.vmLastAssigned)
	if err != nil {
		return nil, fmt.Errorf("could not get IP for VM: %w", err)
	}
	bnm.vmLastAssigned = ipAddr

	tuntapLink := &netlink.Tuntap{
		Mode: unix.IFF_TAP,
		LinkAttrs: netlink.LinkAttrs{
			MasterIndex: bnm.bridgeLinkIdx,
		},
	}
	err = bnm.mainNamespace.LinkAdd(tuntapLink)
	if err != nil {
		return nil, fmt.Errorf("failed to create tap link: %w", err)
	}

	// Set the link up
	// Note: The IP is assigned _by the VM_, not by us.
	err = bnm.mainNamespace.LinkSetUp(tuntapLink)
	if err != nil {
		return nil, fmt.Errorf("failed to set tap link up: %w", err)
	}

	// Attach the whitelisting filter to the TAP interface
	err = bnm.packetFilter.Install(tuntapLink.Attrs().Index, ipAddr.String(), mac.String())
	if err != nil {
		return nil, fmt.Errorf("Failed to install BPF fitering on interface: %w", err)
	}

	// Return details of the TAPInterface.
	return &bnmTAPInterface{
		name:    tuntapLink.Attrs().Name,
		idx:     tuntapLink.Attrs().Index,
		mac:     mac.String(), // for the VM to use
		ip:      ipAddr,       // for the VM to use
		netmask: bnm.vmSubnet.Mask,
		dgw:     bnm.vmRouterAddr,
	}, nil
}
