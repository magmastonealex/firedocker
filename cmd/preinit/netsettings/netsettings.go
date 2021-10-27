// Package netsettings is a small wrapper around netlink to provide some nicer configuration structures.
// This package isn't really suitable for general use, but serves the niche of VM network config well
// since there's nothing complicated.
package netsettings

import (
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// NetlinkHelper is any structure that can handle manipulating interfaces using Netlink types
// (in normal operation, this will usually be a *netlink.Handle)
type NetlinkHelper interface {
	AddrAdd(link netlink.Link, addr *netlink.Addr) error
	LinkByName(name string) (netlink.Link, error)
	LinkSetUp(link netlink.Link) error
	AddrList(link netlink.Link, family int) ([]netlink.Addr, error)
	RouteList(link netlink.Link, family int) ([]netlink.Route, error)
	AddrDel(link netlink.Link, addr *netlink.Addr) error
	RouteDel(route *netlink.Route) error
	RouteAdd(route *netlink.Route) error
}

// RouteConfig represents an (over) simplified understanding of a route.
type RouteConfig struct {
	Gw  string
	Dst string
}

// NetConfig is a structure representing the most basic of network settings available for an interface.
type NetConfig struct {
	IPNet  string
	Routes []RouteConfig
}

// ApplyNetConfig will remove all routes & IPs from ifaceName before applying the configuration in the supplied config struct.
func ApplyNetConfig(ifaceName string, config NetConfig) error {
	handle, err := netlink.NewHandle(unix.NETLINK_ROUTE)
	if err != nil {
		return fmt.Errorf("failed to make netlink handle: %v", err)
	}
	return ApplyNetConfigWithHelper(ifaceName, config, handle)
}

// ApplyNetConfigWithHelper will remove all routes & IPs from ifaceName before applying the configuration in the supplied config struct.
// it will use the provided NetlinkHelper to do it's work.
func ApplyNetConfigWithHelper(ifaceName string, config NetConfig, ns NetlinkHelper) error {
	ifce, err := ns.LinkByName(ifaceName)
	if err != nil {
		return fmt.Errorf("failed to find ifce: %v", err)
	}

	err = ns.LinkSetUp(ifce)
	if err != nil {
		return fmt.Errorf("failed to set ifce up: %v", err)
	}

	addrs, err := ns.AddrList(ifce, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("Failed to list addresses: %v", err)
	}

	for _, addr := range addrs {
		err := ns.AddrDel(ifce, &addr)
		if err != nil {
			return fmt.Errorf("Failed to remove address: %v", err)
		}
	}

	routes, err := ns.RouteList(ifce, netlink.FAMILY_ALL)
	if err != nil {
		return fmt.Errorf("Failed listing routes: %v", err)
	}

	for _, route := range routes {
		err := ns.RouteDel(&route)
		if err != nil {
			return fmt.Errorf("Failed deleting route: %v", err)
		}
	}

	addr, err := netlink.ParseAddr(config.IPNet)
	if err != nil {
		return fmt.Errorf("failed to make addr: %v", err)
	}

	err = ns.AddrAdd(ifce, addr)
	if err != nil {
		return fmt.Errorf("failed to add addr: %v", err)
	}

	for _, route := range config.Routes {
		_, rteDst, _ := net.ParseCIDR(route.Dst)
		rte := &netlink.Route{
			LinkIndex: ifce.Attrs().Index,
			Dst:       rteDst,
			Gw:        net.ParseIP(route.Gw),
		}
		err = ns.RouteAdd(rte)
		if err != nil {
			return fmt.Errorf("failed to add route: %v", err)
		}
	}

	// Add route for mmds
	_, mmdsNet, _ := net.ParseCIDR("169.254.169.254/32")
	err = ns.RouteAdd(&netlink.Route{
		LinkIndex: ifce.Attrs().Index,
		Dst:       mmdsNet,
	})
	if err != nil {
		return fmt.Errorf("failed to add mmds route")
	}
	return nil
}
