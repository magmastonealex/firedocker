package networking

func (bnm *bridgingNetManager) GetVMNetns() string {
	return netNSName
}

func (bnm *bridgingNetManager) ReleaseTap(ifce TAPInterface) error {
	// Try to cast it back to a bnm type.
	// Use netns netlink to delete the TAP device
	// Free the IP address assignment.
	return nil
}

func (bnm *bridgingNetManager) CreateTap() (TAPInterface, error) {
	// Create tuntap device
	// Open the Link using netlink
	// Attach the whitelisting filter to the TAP interface
	// Move the TAP interface into the netns
	// Using the other netns, assign the TAP interface an IP.
	// Attach the TAP interface to the bridge.
	// Set up whitelisting for our assigned MAC & IP.
	// Return details of the TAPInterface.
	return nil, nil
}
