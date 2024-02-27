// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"fmt"
	"net"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

// ParseCIDR parses subnet string as a CIDR notation IP address of the
// specified IP version, like "172.18.40.0/24" or "fd00:db8::/32".
func ParseCIDR(version types.IPVersion, subnet string) (*net.IPNet, error) {
	if err := IsCIDR(version, subnet); err != nil {
		return nil, err
	}
	_, ipNet, _ := net.ParseCIDR(subnet)

	return ipNet, nil
}

// ContainsCIDR reports whether subnet1 includes subnet2. Both of them
// are parsed from subnet strings and must belong to the same IP version.
func ContainsCIDR(version types.IPVersion, subnet1 string, subnet2 string) (bool, error) {
	if err := IsIPVersion(version); err != nil {
		return false, err
	}
	if err := IsCIDR(version, subnet1); err != nil {
		return false, err
	}
	if err := IsCIDR(version, subnet2); err != nil {
		return false, err
	}

	return containsCIDR(subnet1, subnet2), nil
}

// IsCIDROverlap reports whether the subnets of specific IP version
// parsed from two subnet strings overlap.
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

	return containsCIDR(subnet1, subnet2) || containsCIDR(subnet2, subnet1), nil
}

func containsCIDR(subnet1 string, subnet2 string) bool {
	// Ignore the error returned here. The format of the subnet should be
	// verified in external IsCIDR.
	_, ipNet1, _ := net.ParseCIDR(subnet1)
	_, ipNet2, _ := net.ParseCIDR(subnet2)
	ones1, _ := ipNet1.Mask.Size()
	ones2, _ := ipNet2.Mask.Size()
	if ones1 <= ones2 && ipNet1.Contains(ipNet2.IP) {
		return true
	}

	return false
}

// IsCIDR reports whether subnet string is a CIDR notation IP address
// of the specified IP version.
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

// IsIPv4CIDR reports whether subnet string is a CIDR notation IPv4 address.
func IsIPv4CIDR(subnet string) bool {
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return false
	}

	return ip.To4() != nil
}

// IsIPv6CIDR reports whether subnet string is a CIDR notation IPv6 address.
func IsIPv6CIDR(subnet string) bool {
	ip, _, err := net.ParseCIDR(subnet)
	if err != nil {
		return false
	}

	return ip.To4() == nil
}

func IsFormatCIDR(subnet string) error {
	_, ipNet, err := net.ParseCIDR(subnet)
	if nil != err {
		return err
	}

	formatCIDR := ipNet.String()

	if formatCIDR != subnet {
		return fmt.Errorf("unformatted subnet %s, please change it to %s", subnet, formatCIDR)
	}

	return nil
}
