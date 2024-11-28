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
func IPsDiffSet(ipSourceList, ipExcludeList []net.IP, sorted bool) []net.IP {
	return getIPDiffSet(ipSourceList, ipExcludeList, sorted, -1)
}

func IsDiffIPSet(ipSourceList, ipExcludeList []net.IP) bool {
	diff := getIPDiffSet(ipSourceList, ipExcludeList, false, 1)
	return len(diff) > 0
}

// getIPDiffSet returns a list of IPs from ipSourceList that are not in ipExcludeList. Parameters:
// - ipSourceList: a slice of net.IP that represents the source list of IPs.
// - ipExcludeList: a slice of net.IP that represents the list of IPs to be excluded.
// - sorted: a boolean indicating whether the resulting list should be sorted.
// - expectCount: an integer specifying the maximum number of IPs to return. If expectCount <= 0, all IPs will be returned.
func getIPDiffSet(ipSourceList, ipExcludeList []net.IP, sorted bool, expectCount int) []net.IP {
	ips2Map := make(map[[16]byte]struct{}, len(ipExcludeList))
	for _, ip := range ipExcludeList {
		if ip != nil {
			ips2Map[[16]byte(ip.To16())] = struct{}{}
		}
	}

	var result []net.IP
	for _, ip := range ipSourceList {
		if ip != nil {
			if _, ok := ips2Map[[16]byte(ip.To16())]; !ok {
				result = append(result, ip)
				if expectCount > 0 && len(result) >= expectCount {
					break
				}
			}
		}
	}

	if sorted && len(result) > 1 {
		sort.Slice(result, func(i, j int) bool {
			return bytes.Compare(result[i].To16(), result[j].To16()) < 0
		})
	}

	return result
}

// FindAvailableIPs find available ip list in range
func FindAvailableIPs(ipRanges []string, ipList []net.IP, count int) ([]net.IP, error) {
	if count < 0 {
		return nil, fmt.Errorf("count must be non-negative")
	}
	
	if len(ipRanges) == 0 {
		return nil, fmt.Errorf("ipRanges cannot be empty")
	}

	// Use efficient map with [16]byte key for faster lookups
	ipMap := make(map[[16]byte]struct{}, len(ipList))
	ipVersion := -1 // -1: unset, 4: IPv4, 6: IPv6
	
	// Validate and store existing IPs
	for _, ip := range ipList {
		if ip == nil {
			continue
		}
		
		// Determine and validate IP version consistency
		if ip.To4() != nil {
			if ipVersion == 6 {
				return nil, fmt.Errorf("mixed IPv4 and IPv6 addresses are not supported")
			}
			ipVersion = 4
		} else {
			if ipVersion == 4 {
				return nil, fmt.Errorf("mixed IPv4 and IPv6 addresses are not supported")
			}
			ipVersion = 6
		}
		ipMap[[16]byte(ip.To16())] = struct{}{}
	}

	var availableIPs []net.IP
	
	// Process each IP range
	for _, ipRange := range ipRanges {
		if count == 0 {
			break
		}

		ips := strings.Split(ipRange, "-")
		if len(ips) > 2 {
			return nil, fmt.Errorf("invalid IP range format: %s", ipRange)
		}

		startIP := net.ParseIP(ips[0])
		if startIP == nil {
			return nil, fmt.Errorf("invalid start IP: %s", ips[0])
		}

		var endIP net.IP
		if len(ips) == 2 {
			endIP = net.ParseIP(ips[1])
			if endIP == nil {
				return nil, fmt.Errorf("invalid end IP: %s", ips[1])
			}
		} else {
			endIP = startIP
		}

		// Validate IP version consistency within range
		if (startIP.To4() != nil) != (endIP.To4() != nil) {
			return nil, fmt.Errorf("IP range %s contains mixed IPv4 and IPv6 addresses", ipRange)
		}

		// Validate IP version consistency with existing IPs
		if ipVersion != -1 {
			if (startIP.To4() != nil) != (ipVersion == 4) {
				return nil, fmt.Errorf("IP range %s version mismatch with existing IPs", ipRange)
			}
		} else {
			ipVersion = map[bool]int{true: 4, false: 6}[startIP.To4() != nil]
		}

		// Validate range order
		if bytes.Compare(startIP, endIP) > 0 {
			return nil, fmt.Errorf("invalid IP range: start IP %s is greater than end IP %s", startIP, endIP)
		}

		// Find available IPs in range using efficient lookup
		stop := nextIP(endIP)
		for ip := startIP; !ip.Equal(stop) && count > 0; ip = nextIP(ip) {
			if _, exists := ipMap[[16]byte(ip.To16())]; !exists {
				newIP := make(net.IP, len(ip))
				copy(newIP, ip)
				availableIPs = append(availableIPs, newIP)
				ipMap[[16]byte(ip.To16())] = struct{}{} // Prevent duplicates across ranges
				count--
			}
		}
	}

	return availableIPs, nil
}

func nextIP(ip net.IP) net.IP {
	next := make(net.IP, len(ip))
	copy(next, ip)

	for i := len(next) - 1; i >= 0; i-- {
		next[i]++
		if next[i] != 0 {
			break
		}
	}
	return next
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
