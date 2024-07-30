// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"fmt"
	"math"
	"math/big"
	"net"
	"strings"
)

type Range struct {
	Raw        string
	Start, End *big.Int
}

type CIDR struct {
	base    net.IPNet           // base is the base subnet
	isIPv4  bool                // isIPv4 is true if the base subnet is an IPv4 address
	ranges  []Range             // ranges is available IP ranges
	usedMap map[string]struct{} // usedMap is a map of used IP addresses
}

func NewCIDR(base string, ranges []string, exclude []string) (*CIDR, error) {
	_, ipNet, err := net.ParseCIDR(base)
	if err != nil {
		return nil, err
	}
	subnet := &CIDR{base: *ipNet}
	if ipNet.IP.To4() != nil {
		subnet.isIPv4 = true
	}

	for _, item := range ranges {
		s, e, err := subnet.parse(item)
		if err != nil {
			return nil, err
		}
		err = subnet.addIncludeRange(Range{Raw: item, Start: s, End: e})
		if err != nil {
			return nil, err
		}
	}

	excludeRanges := make([]Range, 0)
	for _, item := range exclude {
		s, e, err := subnet.parse(item)
		if err != nil {
			return nil, err
		}
		r := Range{Raw: item, Start: s, End: e}
		excludeRanges = append(excludeRanges, r)
	}
	subnet.ranges = removeExcludeIPRange(subnet.ranges, excludeRanges)
	return subnet, nil
}

// AddUsedIP adds the IP addresses to the used store.
func (c *CIDR) AddUsedIP(list ...string) error {
	for _, value := range list {
		c.usedMap[value] = struct{}{}
	}
	return nil
}

// TotalUsedIPInt returns the number of used IP addresses in the subnet.
func (c *CIDR) TotalUsedIPInt() int {
	return len(c.usedMap)
}

// TotalIP returns the number of IP addresses in the subnet.
func (c *CIDR) TotalIP() *big.Int {
	total := big.NewInt(0)
	for _, r := range c.ranges {
		total.Add(total, big.NewInt(1))
		total.Add(total, new(big.Int).Sub(r.End, r.Start))
	}
	return total
}

// TotalIPInt returns the number of IP addresses in the subnet as an integer.
func (c *CIDR) TotalIPInt() int {
	totalInt64 := c.TotalIP().Int64()
	if c.TotalIP().Cmp(big.NewInt(math.MaxInt64)) > 0 {
		return math.MaxInt64
	}
	return int(totalInt64)
}

// addIncludeRange adds the range to the subnet.
func (c *CIDR) addIncludeRange(r Range) error {
	if r.Start == nil || r.End == nil {
		return fmt.Errorf("nil range: %s", r.Raw)
	}
	// if start > end, start = end, end = start
	if r.Start.Cmp(r.End) > 0 {
		r.Start, r.End = r.End, r.Start
	}
	a := Range{Raw: r.Raw, Start: r.Start, End: r.End}
	for _, b := range c.ranges {
		// a.start in b
		if a.Start.Cmp(b.Start) >= 0 && a.End.Cmp(b.End) <= 0 {
			return fmt.Errorf("range %v is already covered by %v", a.Raw, b.Raw)
		}
		// a.end in b
		if a.Start.Cmp(b.Start) <= 0 && a.End.Cmp(b.End) >= 0 {
			return fmt.Errorf("range %v covers %v", a.Raw, b.Raw)
		}
		// b.start in a
		if a.Start.Cmp(b.Start) >= 0 && a.Start.Cmp(b.End) <= 0 {
			return fmt.Errorf("range %v overlaps with %v", a.Raw, b.Raw)
		}
		// b.end in a
		if a.End.Cmp(b.Start) >= 0 && a.End.Cmp(b.End) <= 0 {
			return fmt.Errorf("range %v overlaps with %v", a.Raw, b.Raw)
		}
	}
	c.ranges = append(c.ranges, a)
	return nil
}

// parse parses the range string and returns the start and end IP addresses.
// case ipv4
// - "10.6.0.1"
// - "10.6.0.1-10.6.0.1"
// - "10.6.0.1/24"
// case ipv6
// - "fd00:db8::1"
// - "fd00:db8::1-fd00:db8::1"
// - "fd00:db8::1/64"
func (c *CIDR) parse(value string) (*big.Int, *big.Int, error) {
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return nil, nil, fmt.Errorf("invalid range: %v", value)
	}
	if len(parts) == 1 {
		if strings.Contains(value, "/") {
			return nil, nil, fmt.Errorf("invalid range: %v", value)
		} else {
			res, err := c.convertIPtoBigInt(value)
			return res, res, err
		}
	}
	start, err := c.convertIPtoBigInt(parts[0])
	if err != nil {
		return nil, nil, err
	}
	end, err := c.convertIPtoBigInt(parts[1])
	if err != nil {
		return nil, nil, err
	}
	return start, end, nil
}

// convertIPtoBigInt converts the IP address to big.Int.
func (c *CIDR) convertIPtoBigInt(value string) (*big.Int, error) {
	ip := net.ParseIP(value)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %v", value)
	}
	if !c.base.Contains(ip) {
		return nil, fmt.Errorf("not match base subnet: %v", value)
	}
	if c.isIPv4 {
		ipv4 := ip.To4()
		if ipv4 == nil {
			return nil, fmt.Errorf("not match base subnet IPv4 version: %v", value)
		}
		res := big.NewInt(0).SetBytes(ipv4)
		return res, nil
	} else {
		ipv6 := ip.To16()
		if ipv6 == nil {
			return nil, fmt.Errorf("not match base subnet IPv6 version: %v", value)
		}
		res := big.NewInt(0).SetBytes(ipv6)
		return res, nil
	}
}

// IPRange returns the IP ranges in the subnet.
func (c *CIDR) IPRange() []Range {
	return c.ranges
}

// IsOverlapIPRanges checks if the subnet overlaps with the given ranges.
func (c *CIDR) IsOverlapIPRanges(r []Range) ([]string, bool) {
	overlaps := make([]string, 0)
	for _, x := range c.ranges {
		for _, y := range r {
			if (x.Start.Cmp(y.Start) >= 0 && x.Start.Cmp(y.End) <= 0) ||
				(x.End.Cmp(y.Start) >= 0 && x.End.Cmp(y.End) <= 0) ||
				(y.Start.Cmp(x.Start) >= 0 && y.Start.Cmp(x.End) <= 0) ||
				(y.End.Cmp(x.Start) >= 0 && y.End.Cmp(x.End) <= 0) {
				overlaps = append(overlaps, y.Raw)
			}
		}
	}
	return overlaps, len(overlaps) > 0
}

func removeExcludeIPRange(base []Range, exclude []Range) []Range {
	f := func(base []Range, toBeRemoved Range) (res []Range) {
		x, y := toBeRemoved.Start, toBeRemoved.End
		for _, e := range base {
			a, b := e.Start, e.End
			if a.Cmp(y) > 0 || b.Cmp(x) < 0 {
				res = append(res, e)
			} else {
				if a.Cmp(x) < 0 {
					newEnd := new(big.Int).Sub(x, big.NewInt(1))
					res = append(res, Range{e.Raw, a, newEnd})
				}
				if b.Cmp(y) > 0 {
					newStart := new(big.Int).Add(y, big.NewInt(1))
					res = append(res, Range{e.Raw, newStart, b})
				}
			}
		}
		return
	}
	result := base
	for _, e := range exclude {
		result = f(result, e)
	}
	return result
}
