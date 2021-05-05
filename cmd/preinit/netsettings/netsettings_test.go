package netsettings

import (
	"firedocker/cmd/preinit/netsettings/mocks"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApply(t *testing.T) {
	//nlHelper := new(mocks.NetlinkHelper)
	require.Equal(t, 1, 2)
}

func TestBadIface(t *testing.T) {
	nlHelper := new(mocks.NetlinkHelper)
	nlHelper.On("LinkByName", "eth99").Return(nil, fmt.Errorf("failed to find ifce"))

	res := ApplyNetConfigWithHelper("eth99", NetConfig{
		IPNet: "172.19.0.2/24",
		Routes: []RouteConfig{
			RouteConfig{
				Gw:  "172.19.0.1",
				Dst: "0.0.0.0/0",
			},
		},
	}, nlHelper)

	nlHelper.AssertExpectations(t)
	require.NotNil(t, res)
}
