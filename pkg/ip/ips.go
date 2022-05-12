// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"math/big"
	"net"
	"strings"
)

type IPs []net.IP

func ParseIPRanges(ipRanges []string) IPs {
	var ips IPs
	for _, r := range ipRanges {
		ips = append(ips, parseIPRange(r)...)
	}

	return ips
}

func parseIPRange(ipRange string) IPs {
	var ips IPs
	arr := strings.Split(ipRange, "-")
	if len(arr) == 2 {
		cur := net.ParseIP(arr[0])
		end := net.ParseIP(arr[1])
		for Cmp(cur, end) <= 0 {
			ips = append(ips, cur)
			cur = NextIP(cur)
		}
	} else {
		ips = append(ips, net.ParseIP(arr[0]))
	}

	return ips
}

func IPsDiffSet(src, target IPs) IPs {
	var ips IPs
	marks := make(map[string]bool)
	for _, ip := range src {
		if _, ok := marks[ip.String()]; !ok {
			marks[ip.String()] = true
		}
	}

	for _, ip := range target {
		delete(marks, ip.String())
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
