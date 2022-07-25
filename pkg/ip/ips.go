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
)

var (
	ErrInvalidIPRangeFormat = errors.New("invalid IP range format")
	ErrInvalidIPFormat      = errors.New("invalid IP format")
	ErrInvalidCIDRFormat    = errors.New("invalid CIDR format")
)

func ParseIPRanges(ipRanges []string) ([]net.IP, error) {
	var sum []net.IP
	for _, r := range ipRanges {
		ips, err := parseIPRange(r)
		if err != nil {
			return nil, err
		}
		sum = append(sum, ips...)
	}

	return sum, nil
}

func parseIPRange(ipRange string) ([]net.IP, error) {
	if !IsIPRange(ipRange) {
		return nil, fmt.Errorf("%w '%s'", ErrInvalidIPRangeFormat, ipRange)
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

func IPsDiffSet(a, b []net.IP) []net.IP {
	var ips []net.IP
	marks := make(map[string]bool)
	for _, ip := range a {
		if ip != nil {
			marks[ip.String()] = true
		}
	}

	for _, ip := range b {
		if ip != nil {
			delete(marks, ip.String())
		}
	}

	for k := range marks {
		ips = append(ips, net.ParseIP(k))
	}

	return ips
}

func ParseIP(s string) (*net.IPNet, error) {
	if govalidator.IsCIDR(s) {
		ip, ipNet, _ := net.ParseCIDR(s)
		return &net.IPNet{IP: ip, Mask: ipNet.Mask}, nil
	}

	if govalidator.IsIP(s) {
		return &net.IPNet{IP: net.ParseIP(s)}, nil
	}

	return nil, fmt.Errorf("%w '%s'", ErrInvalidIPFormat, s)
}

// IsIPRange check the format for the given ip range.
// it can be a single one just like '192.168.1.0',
// and it also could be an IP range just like '192.168.1.0-192.168.1.10'.
// Notice: the following formats are invalid
// 1. '192.168.1.0 - 192.168.1.10', there can not be space between two IP.
// 2. '192.168.1.1-2001:db8:a0b:12f0::1', the combination with one IPv4 and IPv6 is invalid.
// 3. '192.168.1.10-192.168.1.1', the IP range must be ordered.
func IsIPRange(ipRange string) bool {
	return IsIPv4IPRange(ipRange) || IsIPv6IPRange(ipRange)
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

func CheckIPv4CIDROverlap(a, b string) (bool, error) {
	if !IsIPv4CIDR(a) {
		return false, fmt.Errorf("%w in IPv4 '%s'", ErrInvalidCIDRFormat, a)
	}
	if !IsIPv4CIDR(b) {
		return false, fmt.Errorf("%w in IPv4 '%s'", ErrInvalidCIDRFormat, b)
	}
	_, ipNet1, _ := net.ParseCIDR(a)
	_, ipNet2, _ := net.ParseCIDR(b)

	return isCIDROverlap(ipNet1, ipNet2), nil
}

func CheckIPv6CIDROverlap(a, b string) (bool, error) {
	if !IsIPv6CIDR(a) {
		return false, fmt.Errorf("%w in IPv6 '%s'", ErrInvalidCIDRFormat, a)
	}
	if !IsIPv6CIDR(b) {
		return false, fmt.Errorf("%w in IPv6 '%s'", ErrInvalidCIDRFormat, b)
	}
	_, ipNet1, _ := net.ParseCIDR(a)
	_, ipNet2, _ := net.ParseCIDR(b)

	return isCIDROverlap(ipNet1, ipNet2), nil
}

func isCIDROverlap(a, b *net.IPNet) bool {
	ones1, _ := a.Mask.Size()
	ones2, _ := b.Mask.Size()
	if ones1 < ones2 && a.Contains(b.IP) {
		return true
	}
	if ones1 > ones2 && a.Contains(b.IP) {
		return true
	}

	return false
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

func NextIP(ip net.IP) net.IP {
	i := ipToInt(ip)
	return intToIP(i.Add(i, big.NewInt(1)))
}

func PrevIP(ip net.IP) net.IP {
	i := ipToInt(ip)
	return intToIP(i.Sub(i, big.NewInt(1)))
}

// Cmp compares two IPs, returning the usual ordering:
// a < b : -1
// a == b : 0
// a > b : 1
func Cmp(a, b net.IP) int {
	aa := ipToInt(a)
	bb := ipToInt(b)
	return aa.Cmp(bb)
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
