package networking

import (
	"fmt"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"net"
	"regexp"
	"strings"
)

var DefaultInterfacesToExclude = []string{
	"docker.*", "cbr.*", "dummy.*",
	"virbr.*", "lxcbr.*", "veth.*", "lo",
	"cali.*", "tunl.*", "flannel.*", "kube-ipvs.*",
	"cni.*", "vx-submariner", "cilium*",
}

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
func IPAddressOnNode(logger *zap.Logger, ipFamily int) ([]netlink.Addr, error) {
	var err error
	var excludeRegexp *regexp.Regexp
	if excludeRegexp, err = regexp.Compile("(" + strings.Join(DefaultInterfacesToExclude, ")|(") + ")"); err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	links, err := netlink.LinkList()
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	var allIPAddress []netlink.Addr
	for idx, _ := range links {
		iLink := links[idx]
		if excludeRegexp.MatchString(iLink.Attrs().Name) {
			continue
		}

		ipAddress, err := GetAddersByLink(iLink, ipFamily)
		if err != nil {
			logger.Error(err.Error())
			return nil, err
		}
		allIPAddress = append(allIPAddress, ipAddress...)
	}
	logger.Debug("Get IPAddressOnNode", zap.Any("allIPAddress", allIPAddress))
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
	addres, err := netlink.AddrList(link, ipfamily)
	if err != nil {
		return nil, err
	}

	for _, addr := range addres {
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

// AddrToString convert obj netlink.addr to ip's string list(mask 32)
func AddrToString(addrs []netlink.Addr) []string {
	addrStrings := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		if addr.IP.To4() != nil {
			addr.IPNet.Mask = net.CIDRMask(32, 32)
		} else {
			addr.IPNet.Mask = net.CIDRMask(128, 128)
		}
		addrStrings = append(addrStrings, addr.IPNet.String())
	}
	return addrStrings
}

func IsInterfaceMiss(netns ns.NetNS, iface string) (bool, error) {
	err := netns.Do(func(_ ns.NetNS) error {
		_, err := netlink.LinkByName(iface)
		return err
	})

	if err == nil {
		return false, nil
	}

	if strings.EqualFold(err.Error(), ip.ErrLinkNotFound.Error()) {
		return true, nil
	} else {
		return false, err
	}
}
