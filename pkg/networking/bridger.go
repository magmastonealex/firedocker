package networking

func (bnm *bridgingNetManager) GetVMNetns() string {
	return netNSName
}

func (bnm *bridgingNetManager) ReleaseTap(ifce TAPInterface) error {

}

func (bnm *bridgingNetManager) CreateTap() (TAPInterface, error) {

}
