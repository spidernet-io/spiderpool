// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/asaskevich/govalidator"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var (
	ErrInvalidIPVersion     = errors.New("invalid IP version")
	ErrInvalidIPRangeFormat = errors.New("invalid IP range format")
	ErrInvalidIPFormat      = errors.New("invalid IP format")
	ErrInvalidCIDRFormat    = errors.New("invalid CIDR format")
)

func IsIPVersion(version types.IPVersion) error {
	if version != constant.IPv4 && version != constant.IPv6 {
		return fmt.Errorf("%w '%d'", ErrInvalidIPVersion, version)
	}

	return nil
}

func ParseIPRanges(version types.IPVersion, ipRanges []string) ([]net.IP, error) {
	var sum []net.IP
	for _, r := range ipRanges {
		ips, err := ParseIPRange(version, r)
		if err != nil {
			return nil, err
		}
		sum = append(sum, ips...)
	}

	return sum, nil
}

func ParseIPRange(version types.IPVersion, ipRange string) ([]net.IP, error) {
	if err := IsIPRange(version, ipRange); err != nil {
		return nil, err
	}

	arr := strings.Split(ipRange, "-")
	n := len(arr)
	var ips []net.IP
	if n == 1 {
		ips = append(ips, net.ParseIP(arr[0]))
	}

	if n == 2 {
		cur := net.ParseIP(arr[0])
		end := net.ParseIP(arr[1])
		for Cmp(cur, end) <= 0 {
			ips = append(ips, cur)
			cur = NextIP(cur)
		}
	}

	return ips, nil
}

func ContainsIPRange(version types.IPVersion, subnet string, ipRange string) (bool, error) {
	ipNet, err := ParseCIDR(version, subnet)
	if err != nil {
		return false, err
	}
	ips, err := ParseIPRange(version, ipRange)
	if err != nil {
		return false, err
	}

	n := len(ips)
	if n == 1 {
		return ipNet.Contains(ips[0]), nil
	}

	return ipNet.Contains(ips[0]) && ipNet.Contains(ips[n-1]), nil
}

func IsIPRangeOverlap(version types.IPVersion, ipRange1, ipRange2 string) (bool, error) {
	if err := IsIPVersion(version); err != nil {
		return false, err
	}
	if err := IsIPRange(version, ipRange1); err != nil {
		return false, err
	}
	if err := IsIPRange(version, ipRange2); err != nil {
		return false, err
	}

	ips1, _ := ParseIPRange(version, ipRange1)
	ips2, _ := ParseIPRange(version, ipRange2)
	if len(ips1) > len(IPsDiffSet(ips1, ips2)) {
		return true, nil
	}

	return false, nil
}

// IsIPRange check the format for the given ip range.
// it can be a single one just like '192.168.1.0',
// and it also could be an IP range just like '192.168.1.0-192.168.1.10'.
// Notice: the following formats are invalid
// 1. '192.168.1.0 - 192.168.1.10', there can not be space between two IP.
// 2. '192.168.1.1-2001:db8:a0b:12f0::1', the combination with one IPv4 and IPv6 is invalid.
// 3. '192.168.1.10-192.168.1.1', the IP range must be ordered.
func IsIPRange(version types.IPVersion, ipRange string) error {
	if err := IsIPVersion(version); err != nil {
		return err
	}

	if (version == constant.IPv4 && !IsIPv4IPRange(ipRange)) ||
		(version == constant.IPv6 && !IsIPv6IPRange(ipRange)) {
		return fmt.Errorf("%w in IPv%d '%s'", ErrInvalidIPRangeFormat, version, ipRange)
	}

	return nil
}

func IsIPv4IPRange(ipRange string) bool {
	ips := strings.Split(ipRange, "-")
	n := len(ips)
	if n > 2 {
		return false
	}

	if n == 1 {
		return govalidator.IsIPv4(ips[0])
	}

	if n == 2 {
		if !govalidator.IsIPv4(ips[0]) || !govalidator.IsIPv4(ips[1]) {
			return false
		}
		if Cmp(net.ParseIP(ips[0]), net.ParseIP(ips[1])) == 1 {
			return false
		}
	}

	return true
}

func IsIPv6IPRange(ipRange string) bool {
	ips := strings.Split(ipRange, "-")
	n := len(ips)
	if n > 2 {
		return false
	}

	if n == 1 {
		return govalidator.IsIPv6(ips[0])
	}

	if n == 2 {
		if !govalidator.IsIPv6(ips[0]) || !govalidator.IsIPv6(ips[1]) {
			return false
		}
		if Cmp(net.ParseIP(ips[0]), net.ParseIP(ips[1])) == 1 {
			return false
		}
	}

	return true
}

func ParseIP(version types.IPVersion, s string) (*net.IPNet, error) {
	if strings.ContainsAny(s, "/") {
		if err := IsCIDR(version, s); err != nil {
			return nil, err
		}
		ip, ipNet, _ := net.ParseCIDR(s)
		return &net.IPNet{IP: ip, Mask: ipNet.Mask}, nil
	} else {
		if err := IsIP(version, s); err != nil {
			return nil, err
		}
		return &net.IPNet{IP: net.ParseIP(s)}, nil
	}
}

func ContainsIP(version types.IPVersion, subnet string, ip string) (bool, error) {
	ipNet, err := ParseCIDR(version, subnet)
	if err != nil {
		return false, err
	}
	address, err := ParseIP(version, ip)
	if err != nil {
		return false, err
	}

	return ipNet.Contains(address.IP), nil
}

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

func ParseCIDR(version types.IPVersion, subnet string) (*net.IPNet, error) {
	if err := IsCIDR(version, subnet); err != nil {
		return nil, err
	}
	_, ipNet, _ := net.ParseCIDR(subnet)

	return ipNet, nil
}

func IsCIDROverlap(version types.IPVersion, subnet1, subnet2 string) (bool, error) {
	if err := IsIPVersion(version); err != nil {
		return false, err
	}
	if err := IsCIDR(version, subnet1); err != nil {
		return false, err
	}
	if err := IsCIDR(version, subnet2); err != nil {
		return false, err
	}

	return isCIDROverlap(subnet1, subnet2), nil
}

func isCIDROverlap(subnet1, subnet2 string) bool {
	_, ipNet1, _ := net.ParseCIDR(subnet1)
	_, ipNet2, _ := net.ParseCIDR(subnet2)
	ones1, _ := ipNet1.Mask.Size()
	ones2, _ := ipNet2.Mask.Size()
	if ones1 < ones2 && ipNet1.Contains(ipNet2.IP) {
		return true
	}
	if ones1 > ones2 && ipNet1.Contains(ipNet2.IP) {
		return true
	}

	return false
}

func IsCIDR(version types.IPVersion, subnet string) error {
	if err := IsIPVersion(version); err != nil {
		return err
	}

	if (version == constant.IPv4 && !IsIPv4CIDR(subnet)) ||
		(version == constant.IPv6 && !IsIPv6CIDR(subnet)) {
		return fmt.Errorf("%w in IPv%d '%s'", ErrInvalidCIDRFormat, version, subnet)
	}

	return nil
}

func IsIPv4CIDR(subnet string) bool {
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return false
	}

	return ip.To4() != nil
}

func IsIPv6CIDR(subnet string) bool {
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return false
	}

	return ip.To4() == nil
}

func IPsDiffSet(ips1, ips2 []net.IP) []net.IP {
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

	return ips
}

func NextIP(ip net.IP) net.IP {
	i := ipToInt(ip)
	return intToIP(i.Add(i, big.NewInt(1)))
}

func PrevIP(ip net.IP) net.IP {
	i := ipToInt(ip)
	return intToIP(i.Sub(i, big.NewInt(1)))
}

// Cmp compares two IPs, returning the usual ordering:
// ip1 < ip2 : -1
// ip1 == ip2 : 0
// ip1 > ip2 : 1
func Cmp(ip1, ip2 net.IP) int {
	int1 := ipToInt(ip1)
	int2 := ipToInt(ip2)
	return int1.Cmp(int2)
}

func ipToInt(ip net.IP) *big.Int {
	if v := ip.To4(); v != nil {
		return big.NewInt(0).SetBytes(v)
	}
	return big.NewInt(0).SetBytes(ip.To16())
}

func intToIP(i *big.Int) net.IP {
	return net.IP(i.Bytes())
}
