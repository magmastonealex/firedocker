// Code generated by mockery v0.0.0-dev. DO NOT EDIT.

package mocks

import (
	mock "github.com/stretchr/testify/mock"
	netlink "github.com/vishvananda/netlink"
)

// NetlinkHelper is an autogenerated mock type for the NetlinkHelper type
type NetlinkHelper struct {
	mock.Mock
}

// AddrAdd provides a mock function with given fields: link, addr
func (_m *NetlinkHelper) AddrAdd(link netlink.Link, addr *netlink.Addr) error {
	ret := _m.Called(link, addr)

	var r0 error
	if rf, ok := ret.Get(0).(func(netlink.Link, *netlink.Addr) error); ok {
		r0 = rf(link, addr)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddrDel provides a mock function with given fields: link, addr
func (_m *NetlinkHelper) AddrDel(link netlink.Link, addr *netlink.Addr) error {
	ret := _m.Called(link, addr)

	var r0 error
	if rf, ok := ret.Get(0).(func(netlink.Link, *netlink.Addr) error); ok {
		r0 = rf(link, addr)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// AddrList provides a mock function with given fields: link, family
func (_m *NetlinkHelper) AddrList(link netlink.Link, family int) ([]netlink.Addr, error) {
	ret := _m.Called(link, family)

	var r0 []netlink.Addr
	if rf, ok := ret.Get(0).(func(netlink.Link, int) []netlink.Addr); ok {
		r0 = rf(link, family)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]netlink.Addr)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(netlink.Link, int) error); ok {
		r1 = rf(link, family)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LinkByName provides a mock function with given fields: name
func (_m *NetlinkHelper) LinkByName(name string) (netlink.Link, error) {
	ret := _m.Called(name)

	var r0 netlink.Link
	if rf, ok := ret.Get(0).(func(string) netlink.Link); ok {
		r0 = rf(name)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(netlink.Link)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string) error); ok {
		r1 = rf(name)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// LinkSetUp provides a mock function with given fields: link
func (_m *NetlinkHelper) LinkSetUp(link netlink.Link) error {
	ret := _m.Called(link)

	var r0 error
	if rf, ok := ret.Get(0).(func(netlink.Link) error); ok {
		r0 = rf(link)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RouteAdd provides a mock function with given fields: route
func (_m *NetlinkHelper) RouteAdd(route *netlink.Route) error {
	ret := _m.Called(route)

	var r0 error
	if rf, ok := ret.Get(0).(func(*netlink.Route) error); ok {
		r0 = rf(route)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RouteDel provides a mock function with given fields: route
func (_m *NetlinkHelper) RouteDel(route *netlink.Route) error {
	ret := _m.Called(route)

	var r0 error
	if rf, ok := ret.Get(0).(func(*netlink.Route) error); ok {
		r0 = rf(route)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// RouteList provides a mock function with given fields: link, family
func (_m *NetlinkHelper) RouteList(link netlink.Link, family int) ([]netlink.Route, error) {
	ret := _m.Called(link, family)

	var r0 []netlink.Route
	if rf, ok := ret.Get(0).(func(netlink.Link, int) []netlink.Route); ok {
		r0 = rf(link, family)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]netlink.Route)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(netlink.Link, int) error); ok {
		r1 = rf(link, family)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}