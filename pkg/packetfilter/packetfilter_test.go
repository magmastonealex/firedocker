package packetfilter

import (
	"bytes"
	"firedocker/pkg/packetfilter/mocks"
	"os"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

//go:generate mockery --name=netlinkHelper --structname=NetlinkHelper
//go:generate mockery --name=tcHelper --structname=TCHelper
//go:generate mockery --name=bpfOpener --structname=BPFOpener
//go:generate mockery --dir=../bpfmap --name=BPFMap

type fakeLink struct {
	attrs *netlink.LinkAttrs
	typ   string
}

func (fl *fakeLink) Attrs() *netlink.LinkAttrs {
	return fl.attrs
}

func (fl *fakeLink) Type() string {
	return fl.typ
}

type testHelperStruct struct {
	whitelister *DefaultPacketWhitelister
	nlHelper    *mocks.NetlinkHelper
	tcHelper    *mocks.TCHelper
	bpfHelper   *mocks.BPFOpener
}

func getInitializedWhitelister() *testHelperStruct {
	nlHelper := new(mocks.NetlinkHelper)
	tcHelper := new(mocks.TCHelper)
	bpfHelper := new(mocks.BPFOpener)

	return &testHelperStruct{
		whitelister: &DefaultPacketWhitelister{
			nlHelper:  nlHelper,
			tcHelper:  tcHelper,
			bpfOpener: bpfHelper.Execute,
		},
		nlHelper:  nlHelper,
		tcHelper:  tcHelper,
		bpfHelper: bpfHelper,
	}
}

func TestInstallAndUpdate(t *testing.T) {
	helperStruct := getInitializedWhitelister()
	helperStruct.nlHelper.On("LinkByIndex", 3).Return(&fakeLink{
		typ: "fakeLink",
		attrs: &netlink.LinkAttrs{
			Name: "fake1",
		},
	}, nil)

	helperStruct.tcHelper.On("EnsureQdiscClsact", "fake1").Return(nil)
	helperStruct.tcHelper.On("LoadBPFIngress", "fake1", mock.MatchedBy(func(filename string) bool {
		byts, err := os.ReadFile(filename)
		if err != nil {
			return false
		}
		return bytes.Equal(byts, bpfFilterContents)
	})).Return(nil)

	fakeIPMap := new(mocks.BPFMap)
	fakeMacMap := new(mocks.BPFMap)

	helperStruct.bpfHelper.On("Execute", "/sys/fs/bpf/tc/globals/ifce_allowed_ip").Return(fakeIPMap, nil)
	helperStruct.bpfHelper.On("Execute", "/sys/fs/bpf/tc/globals/ifce_allowed_macs").Return(fakeMacMap, nil)

	fakeIPMap.On("Close").Return(nil)
	fakeMacMap.On("Close").Return(nil)

	fakeIPMap.On("SetValue", uint32(3), uint64(0x20013ac)).Return(nil)
	fakeMacMap.On("SetValue", uint32(3), uint64(0xaabbccddeeff)).Return(nil)

	res := helperStruct.whitelister.Install(3, "172.19.0.2", "aa:bb:cc:dd:ee:ff")

	require.Nil(t, res)

	helperStruct.nlHelper.AssertExpectations(t)
	helperStruct.tcHelper.AssertExpectations(t)
	helperStruct.bpfHelper.AssertExpectations(t)
	fakeIPMap.AssertExpectations(t)
	fakeMacMap.AssertExpectations(t)
}

func TestUpdateValid(t *testing.T) {
	helperStruct := getInitializedWhitelister()

	fakeIPMap := new(mocks.BPFMap)
	fakeMacMap := new(mocks.BPFMap)

	helperStruct.bpfHelper.On("Execute", "/sys/fs/bpf/tc/globals/ifce_allowed_ip").Return(fakeIPMap, nil)
	helperStruct.bpfHelper.On("Execute", "/sys/fs/bpf/tc/globals/ifce_allowed_macs").Return(fakeMacMap, nil)

	fakeIPMap.On("Close").Return(nil)
	fakeMacMap.On("Close").Return(nil)

	fakeIPMap.On("SetValue", uint32(3), uint64(0xc0e82003)).Return(nil)
	fakeMacMap.On("SetValue", uint32(3), uint64(0x84f6fa0033ab)).Return(nil)

	res := helperStruct.whitelister.UpdateByIndex(3, "3.32.232.192", "84:f6:fa:00:33:ab")

	require.Nil(t, res)

	helperStruct.nlHelper.AssertExpectations(t)
	helperStruct.tcHelper.AssertExpectations(t)
	helperStruct.bpfHelper.AssertExpectations(t)
	fakeIPMap.AssertExpectations(t)
	fakeMacMap.AssertExpectations(t)
}

func TestUpdateInvalidIP(t *testing.T) {
	helperStruct := getInitializedWhitelister()

	res := helperStruct.whitelister.UpdateByIndex(3, "google.com", "84:f6:fa:00:33:ab")

	require.NotNil(t, res)
}
func TestUpdateInvalidIPv6(t *testing.T) {
	helperStruct := getInitializedWhitelister()

	res := helperStruct.whitelister.UpdateByIndex(3, "fe80::19f1:67:adca:3eb3", "84:f6:fa:00:33:ab")

	require.NotNil(t, res)
}

func TestUpdateInvalidMAC(t *testing.T) {
	helperStruct := getInitializedWhitelister()

	res := helperStruct.whitelister.UpdateByIndex(3, "192.168.0.3", "84:f6:fa:00:33:ab:dd:ee:ff:gg")

	require.NotNil(t, res)
}
