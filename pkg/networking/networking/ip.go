// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking

import (
	"fmt"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"net"
	"os"
	"regexp"
	"strings"
)

// GetIPFamilyByResult return IPFamily by parse CNI Result
func GetIPFamilyByResult(prevResult *current.Result) (int, error) {
	if len(prevResult.Interfaces) == 0 {
		return -1, fmt.Errorf("can't found any interface from prevResult")
	}

	ipFamily := -1
	enableIpv4, enableIpv6 := false, false
	for _, v := range prevResult.IPs {
		if v.Address.IP.To4() != nil {
			enableIpv4 = true
			ipFamily = netlink.FAMILY_V4
		} else {
			enableIpv6 = true
			ipFamily = netlink.FAMILY_V6
		}
	}

	if ipFamily < 0 {
		return ipFamily, fmt.Errorf("failed to get pod's ip family: no found ips")
	}

	if enableIpv4 && enableIpv6 {
		return netlink.FAMILY_ALL, nil
	}

	return ipFamily, nil
}

func GetGatewayIP(addrs []netlink.Addr) (v4Gw, v6Gw net.IP, err error) {
	for _, addr := range addrs {
		routes, err := netlink.RouteGet(addr.IP)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to RouteGet Pod IP(%s): %v", addr.IP.String(), err)
		}

		if len(routes) > 0 {
			if addr.IP.To4() != nil && v4Gw == nil {
				v4Gw = routes[0].Src
			}
			if addr.IP.To4() == nil && v6Gw == nil {
				v6Gw = routes[0].Src
			}
		}
	}
	return
}

// IPAddressByName returns all IP addresses of the given pod's interface
// group by ipFamily
func IPAddressByName(netns ns.NetNS, interfacenName string, ipFamily int) ([]netlink.Addr, error) {
	var err error
	ipAddress := make([]netlink.Addr, 0, 2)
	err = netns.Do(func(_ ns.NetNS) error {
		ipAddress, err = GetAddersByName(interfacenName, ipFamily)
		return err
	})

	if err != nil {
		return nil, err
	}
	return ipAddress, nil
}

// IPAddressOnNode return all ip addresses on the node, filter by ipFamily
// skipping any interfaces whose name matches any of the exclusion list regexes
func GetAllIPAddress(ipFamily int, excludeInterface []string) ([]netlink.Addr, error) {
	var err error
	var excludeRegexp *regexp.Regexp

	if excludeInterface != nil {
		if excludeRegexp, err = regexp.Compile("(" + strings.Join(excludeInterface, ")|(") + ")"); err != nil {
			return nil, err
		}
	}

	links, err := netlink.LinkList()
	if err != nil {
		return nil, err
	}

	var allIPAddress []netlink.Addr
	for idx := range links {
		iLink := links[idx]

		if excludeRegexp != nil && excludeRegexp.MatchString(iLink.Attrs().Name) {
			continue
		}

		ipAddress, err := GetAddersByLink(iLink, ipFamily)
		if err != nil {
			return nil, err
		}
		allIPAddress = append(allIPAddress, ipAddress...)
	}
	return allIPAddress, nil
}

// GetAddersByName return all unicast ip address of interface, filter by ipFamily
func GetAddersByName(iface string, ipfamily int) ([]netlink.Addr, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return nil, err
	}
	return getAdders(link, ipfamily)
}

// GetAddersByLink return all unicast ip address of interface, filter by interface,ipFamily
func GetAddersByLink(link netlink.Link, ipfamily int) ([]netlink.Addr, error) {
	return getAdders(link, ipfamily)
}

func getAdders(link netlink.Link, ipfamily int) ([]netlink.Addr, error) {
	var ipAddress []netlink.Addr
	address, err := netlink.AddrList(link, ipfamily)
	if err != nil {
		return nil, err
	}

	for _, addr := range address {
		if addr.IP.IsMulticast() || addr.IP.IsLinkLocalUnicast() {
			continue
		}
		if addr.IP.To4() != nil && (ipfamily == netlink.FAMILY_V4 || ipfamily == netlink.FAMILY_ALL) {
			ipAddress = append(ipAddress, addr)
		}
		if addr.IP.To4() == nil && (ipfamily == netlink.FAMILY_V6 || ipfamily == netlink.FAMILY_ALL) {
			ipAddress = append(ipAddress, addr)
		}
	}
	return ipAddress, nil
}

func CheckInterfaceExist(netns ns.NetNS, iface string) (bool, error) {
	var exist bool
	var err error
	if netns != nil {
		err = netns.Do(func(_ ns.NetNS) error {
			exist, err = isInterfaceExist(iface)
			return err
		})
		return exist, err
	}
	return isInterfaceExist(iface)
}

func isInterfaceExist(iface string) (bool, error) {
	_, err := netlink.LinkByName(iface)
	if err == nil {
		return true, nil
	}

	if _, ok := err.(netlink.LinkNotFoundError); ok {
		return false, nil
	} else {
		return false, err
	}
}

func LinkSetBondSlave(slave string, bond *netlink.Bond) error {
	l, err := netlink.LinkByName(slave)
	if err != nil {
		return fmt.Errorf("failed to LinkByName slave %s: %w", slave, err)
	}

	if err = netlink.LinkSetBondSlave(l, bond); err != nil {
		return fmt.Errorf("failed to LinkSetBondSlave: %w", err)
	}
	return nil
}

func LinkAdd(link netlink.Link) error {
	return linkAddAndSetUp(link)
}

func linkAddAndSetUp(link netlink.Link) error {
	var err error
	if err = netlink.LinkAdd(link); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("failed to LinkAdd %s: %w", link.Attrs().Name, err)
	}

	if err = netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("failed to set %s up: %w", link.Attrs().Name, err)
	}
	return nil
}

// IPNetEqual returns true iff both IPNet are equal
// Copyright Authors of vishvananda/netlink
func IPNetEqual(ipn1 *net.IPNet, ipn2 *net.IPNet) bool {
	if ipn1 == ipn2 {
		return true
	}
	if ipn1 == nil || ipn2 == nil {
		return false
	}
	m1, _ := ipn1.Mask.Size()
	m2, _ := ipn2.Mask.Size()
	return m1 == m2 && ipn1.IP.Equal(ipn2.IP)
}
