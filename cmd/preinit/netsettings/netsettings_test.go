package netsettings

//go:generate mockery --name=NetlinkHelper

import (
	"firedocker/cmd/preinit/netsettings/mocks"
	"fmt"
	"net"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/vishvananda/netlink"
)

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

func TestApply(t *testing.T) {
	nlHelper := new(mocks.NetlinkHelper)

	configToApply := NetConfig{
		IPNet: "172.19.0.2/24",
		Routes: []RouteConfig{
			RouteConfig{
				Gw:  "172.19.0.1",
				Dst: "0.0.0.0/0",
			},
		},
	}

	link := &fakeLink{
		attrs: &netlink.LinkAttrs{
			Index: 5,
		},
		typ: "ethernet or something?",
	}
	nlHelper.On("LinkByName", "eth30").Return(link, nil)
	nlHelper.On("LinkSetUp", link).Return(nil)
	addr1, _ := netlink.ParseAddr("192.168.0.5/24")
	nlHelper.On("AddrList", link, netlink.FAMILY_ALL).Return([]netlink.Addr{*addr1}, nil)
	nlHelper.On("AddrDel", link, addr1).Return(nil)
	route1 := netlink.Route{
		LinkIndex: 5,
	}
	nlHelper.On("RouteList", link, netlink.FAMILY_ALL).Return([]netlink.Route{route1}, nil)
	var routeDeleted *netlink.Route = nil
	nlHelper.On("RouteDel", mock.Anything).Run(func(args mock.Arguments) {
		routeDeleted = args.Get(0).(*netlink.Route)
	}).Return(nil)

	addrAdding, _ := netlink.ParseAddr(configToApply.IPNet)
	nlHelper.On("AddrAdd", link, addrAdding).Return(nil)

	var routeAdded *netlink.Route = nil
	nlHelper.On("RouteAdd", mock.Anything).Run(func(args mock.Arguments) {
		routeAdded = args.Get(0).(*netlink.Route)
	}).Return(nil)

	res := ApplyNetConfigWithHelper("eth30", configToApply, nlHelper)

	require.Nil(t, res)
	require.NotNil(t, routeAdded)
	require.Equal(t, routeAdded.LinkIndex, 5)
	require.Equal(t, routeAdded.Gw, net.ParseIP(configToApply.Routes[0].Gw))
	require.NotNil(t, routeDeleted)
	require.Equal(t, *routeDeleted, route1)

	nlHelper.AssertExpectations(t)
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
