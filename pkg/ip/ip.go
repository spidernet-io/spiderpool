// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"bytes"
	"fmt"
	"math/big"
	"net"
	"net/netip"
	"sort"
	"strings"

	"github.com/asaskevich/govalidator"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

// IsIPVersion reports whether version is a valid IP version (4 or 6).
func IsIPVersion(version types.IPVersion) error {
	if version != constant.IPv4 && version != constant.IPv6 {
		return fmt.Errorf("%w '%d'", ErrInvalidIPVersion, version)
	}

	return nil
}

// ParseIP parses IP string as a CIDR notation IP address of the specified
// IP version.
func ParseIP(version types.IPVersion, s string, isCIDR bool) (*net.IPNet, error) {
	if isCIDR {
		if err := IsCIDR(version, s); err != nil {
			return nil, err
		}
		ip, ipNet, _ := net.ParseCIDR(s)
		return &net.IPNet{IP: ip, Mask: ipNet.Mask}, nil
	} else {
		if err := IsIP(version, s); err != nil {
			return nil, err
		}

		if version == constant.IPv4 {
			return &net.IPNet{IP: net.ParseIP(s), Mask: net.CIDRMask(32, 32)}, nil
		}

		return &net.IPNet{IP: net.ParseIP(s), Mask: net.CIDRMask(128, 128)}, nil
	}
}

// ContainsIP reports whether the subnet parsed from the subnet string
// includes the IP address parsed from the IP string. Both must belong
// to the same IP version.
func ContainsIP(version types.IPVersion, subnet string, ip string) (bool, error) {
	ipNet, err := ParseCIDR(version, subnet)
	if err != nil {
		return false, err
	}
	address, err := ParseIP(version, ip, false)
	if err != nil {
		return false, err
	}

	return ipNet.Contains(address.IP), nil
}

// IsIP reports whether IP string is a IP address of the specified IP version.
func IsIP(version types.IPVersion, s string) error {
	if err := IsIPVersion(version); err != nil {
		return err
	}

	if (version == constant.IPv4 && !govalidator.IsIPv4(s)) ||
		(version == constant.IPv6 && !govalidator.IsIPv6(s)) {
		return fmt.Errorf("%w in IPv%d '%s'", ErrInvalidIPFormat, version, s)
	}

	return nil
}

// IPsDiffSet calculates the difference set of two IP address slices.
// For example, the difference set between [172.18.40.1 172.18.40.2] and
// [172.18.40.2 172.18.40.3] is [172.18.40.1].
//
// If sorted is true, the result set of IP addresses will be sorted.
func IPsDiffSet(ips1, ips2 []net.IP, sorted bool) []net.IP {
	var ips []net.IP
	marks := make(map[string]bool)
	for _, ip := range ips1 {
		if ip != nil {
			marks[ip.String()] = true
		}
	}

	for _, ip := range ips2 {
		if ip != nil {
			delete(marks, ip.String())
		}
	}

	for k := range marks {
		ips = append(ips, net.ParseIP(k))
	}

	if sorted {
		sort.Slice(ips, func(i, j int) bool {
			return bytes.Compare(ips[i].To16(), ips[j].To16()) < 0
		})
	}

	return ips
}

// IPsUnionSet calculates the union set of two IP address slices.
// For example, the union set between [172.18.40.1 172.18.40.2] and
// [172.18.40.2 172.18.40.3] is [172.18.40.1 172.18.40.2 172.18.40.3].
//
// If sorted is true, the result set of IP addresses will be sorted.
func IPsUnionSet(ips1, ips2 []net.IP, sorted bool) []net.IP {
	var ips []net.IP
	marks := make(map[string]bool)
	ips1 = append(ips1, ips2...)
	for _, ip := range ips1 {
		if ip != nil {
			marks[ip.String()] = true
		}
	}

	for k := range marks {
		ips = append(ips, net.ParseIP(k))
	}

	if sorted {
		sort.Slice(ips, func(i, j int) bool {
			return bytes.Compare(ips[i].To16(), ips[j].To16()) < 0
		})
	}

	return ips
}

// IPsIntersectionSet calculates the intersection set of two IP address
// slices. For example, the intersection set between [172.18.40.1 172.18.40.2]
// and [172.18.40.2 172.18.40.3] is [172.18.40.2].
//
// If sorted is true, the result set of IP addresses will be sorted.
func IPsIntersectionSet(ips1, ips2 []net.IP, sorted bool) []net.IP {
	var ips []net.IP
	set := make(map[string]bool)
	for _, ip := range ips1 {
		if ip != nil {
			set[ip.String()] = true
		}
	}

	marks := make(map[string]bool)
	for _, ip := range ips2 {
		if ip != nil && set[ip.String()] {
			marks[ip.String()] = true
		}
	}

	for k := range marks {
		ips = append(ips, net.ParseIP(k))
	}

	if sorted {
		sort.Slice(ips, func(i, j int) bool {
			return bytes.Compare(ips[i].To16(), ips[j].To16()) < 0
		})
	}

	return ips
}

// NextIP returns the next IP address.
func NextIP(ip net.IP) net.IP {
	i := ipToInt(ip)
	return intToIP(i.Add(i, big.NewInt(1)))
}

// PrevIP returns the previous IP address.
func PrevIP(ip net.IP) net.IP {
	i := ipToInt(ip)
	return intToIP(i.Sub(i, big.NewInt(1)))
}

// Cmp compares two IP addresses, returns according to the following rules:
// ip1 < ip2: -1
// ip1 = ip2: 0
// ip1 > ip2: 1
func Cmp(ip1, ip2 net.IP) int {
	int1 := ipToInt(ip1)
	int2 := ipToInt(ip2)
	return int1.Cmp(int2)
}

// ipToInt converts net.IP to big.Int.
func ipToInt(ip net.IP) *big.Int {
	if v := ip.To4(); v != nil {
		return big.NewInt(0).SetBytes(v.To4())
	}
	return big.NewInt(0).SetBytes(ip.To16())
}

// intToIP converts big.Int to net.IP.
func intToIP(i *big.Int) net.IP {
	return net.IP(i.Bytes()).To16()
}

func ParseIPOrCIDR(s string) (netip.Prefix, error) {
	if !strings.Contains(s, "/") {
		nAddr, err := netip.ParseAddr(s)
		if err != nil {
			return netip.Prefix{}, err
		}

		prefix := 32
		if nAddr.Is6() {
			prefix = 128
		}
		return netip.PrefixFrom(nAddr, prefix), nil
	}

	nPrefix, err := netip.ParsePrefix(s)
	if err != nil {
		return netip.Prefix{}, err
	}
	return nPrefix, nil
}
