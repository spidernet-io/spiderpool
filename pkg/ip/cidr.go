// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"fmt"
	"net"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func ParseCIDR(version types.IPVersion, subnet string) (*net.IPNet, error) {
	if err := IsCIDR(version, subnet); err != nil {
		return nil, err
	}
	_, ipNet, _ := net.ParseCIDR(subnet)

	return ipNet, nil
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
	if ones1 > ones2 && ipNet2.Contains(ipNet1.IP) {
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
