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
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func IsIPVersion(version types.IPVersion) error {
	if version != constant.IPv4 && version != constant.IPv6 {
		return fmt.Errorf("%w '%d'", ErrInvalidIPVersion, version)
	}

	return nil
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
