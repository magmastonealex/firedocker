// Package networking implements the networking configurator used by firedocker.
// It allows configuring a setup in which a variety of TAP devices are joined into a bridge,
// in their own netns, with a veth pair joining that netns with the main netns.
// L2 connectivity is not provided between the main netns and VMs.
// To finish things off, you likely want IPTables rules protecting the main netns -
// ensuring that the "VM" subnet is only coming from the veth pair, not any other device.
// Internet routing/NAT may be useful to you as well...
package networking

import "net"

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

// InitializeNetworkManager creates a NetworkManager
type InitializeNetworkManager func(vmSubnet string) (NetworkManager, error)

// CreateBridgingNetworkManager will initialize a NetworkManager
// which creates a bridge in a secondary network namespace.
// A small /31 is carved out at the top of vmSubnet to provide connectivity to/from the main subnet.
func CreateBridgingNetworkManager(vmSubnet string) (NetworkManager, error) {

}

// TAPInterface describes a TAP device, as well as it's MAC & IP assignment
type TAPInterface interface {
	Name() string
	Idx() int
	MAC() string
	IP() net.IP
	Netmask() net.IPMask
}
