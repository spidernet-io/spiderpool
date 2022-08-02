// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"fmt"
	"math/big"
	"net"
	"strings"

	"github.com/asaskevich/govalidator"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

func ParseIPRanges(ipRanges []string) ([]net.IP, error) {
	var sum []net.IP
	for _, r := range ipRanges {
		_, err := ValidateIPRange(r)
		if nil != err {
			return nil, err
		}

		ips := parseIPRange(r)
		sum = append(sum, ips...)
	}

	return sum, nil
}

func parseIPRange(ipRange string) []net.IP {
	var ips []net.IP
	arr := strings.Split(ipRange, "-")
	n := len(arr)

	if n == 1 {
		if ip := net.ParseIP(arr[0]); ip != nil {
			ips = append(ips, ip)
		}
	}

	if n == 2 {
		cur := net.ParseIP(arr[0])
		end := net.ParseIP(arr[1])
		if cur == nil || end == nil {
			return nil
		}
		for Cmp(cur, end) <= 0 {
			ips = append(ips, cur)
			cur = NextIP(cur)
		}
	}

	return ips
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

func ParseIP(s string) *net.IPNet {
	if strings.ContainsAny(s, "/") {
		ip, ipNet, err := net.ParseCIDR(s)
		if err != nil {
			return nil
		}
		return &net.IPNet{IP: ip, Mask: ipNet.Mask}
	} else {
		ip := net.ParseIP(s)
		if ip == nil {
			return nil
		}
		return &net.IPNet{IP: ip}
	}
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

// ValidateIPRange check the format for the given ip range.
// it can be a single one just like '192.168.1.0',
// and it also could be an IP range just like '192.168.1.0-192.168.1.10'.
// Notice: the following formats are invalid
// 1. '192.168.1.0 - 192.168.1.10', there can not be space between two IP.
// 2. '192.168.1.1-2001:db8:a0b:12f0::1', the combination with one IPv4 and IPv6 is invalid.
// 3. '192.168.1.10-192.168.1.1', the IP range must be ordered.
func ValidateIPRange(ipRange string) (ipVersion int, err error) {
	split := strings.Split(ipRange, "-")
	length := len(split)

	// single IP
	if length == 1 {
		if govalidator.IsIPv4(split[0]) {
			return int(constant.IPv4), nil
		}

		if govalidator.IsIPv6(split[0]) {
			return int(constant.IPv6), nil
		}

		return 0, fmt.Errorf("failed to parse IP range '%s' , it's not a regular IP address", split)
	} else if length == 2 {
		// IP range
		if govalidator.IsIPv4(split[0]) && govalidator.IsIPv4(split[1]) {
			// the previous IP can't greater than the latter one
			if Cmp(net.ParseIP(split[0]), net.ParseIP(split[1])) == 1 {
				return 0, fmt.Errorf("IP range '%s' is not regular", ipRange)
			}

			return int(constant.IPv4), nil
		}

		if govalidator.IsIPv6(split[0]) && govalidator.IsIPv6(split[1]) {
			// the previous IP can't greater than the latter one
			if Cmp(net.ParseIP(split[0]), net.ParseIP(split[1])) == 1 {
				return 0, fmt.Errorf("IP range '%s' is not regular", ipRange)
			}

			return int(constant.IPv6), nil
		}

		err = fmt.Errorf("IP range '%s' is not regular", ipRange)
	} else {
		// not a regular IP format
		err = fmt.Errorf("IP range '%s' is not regular", ipRange)
	}

	return
}
