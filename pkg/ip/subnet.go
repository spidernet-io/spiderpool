// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	"fmt"
	"math"
	"math/big"
	"net"
	"strings"
)

func NewSubnet(base string, ranges []string, exclude []string) (*Subnet, error) {
	_, ipNet, err := net.ParseCIDR(base)
	if err != nil {
		return nil, err
	}
	subnet := &Subnet{base: *ipNet, exclusions: make([]Range, 0)}
	if ipNet.IP.To4() != nil {
		subnet.isIPv4 = true
	}

	for _, item := range ranges {
		s, e, err := subnet.parse(item)
		if err != nil {
			return nil, err
		}
		err = subnet.addRange(item, s, e)
		if err != nil {
			return nil, err
		}
	}

	for _, item := range exclude {
		s, e, err := subnet.parse(item)
		if err != nil {
			return nil, err
		}
		r := Range{Raw: item, Start: s, End: e}
		subnet.exclusions = append(subnet.exclusions, r)
	}
	subnet.ranges = removeExcludeRange(subnet.ranges, subnet.exclusions)
	return subnet, nil
}

type Subnet struct {
	base       net.IPNet
	isIPv4     bool
	ranges     []Range
	exclusions []Range
	usedStore  map[string]struct{}
}

// AddUsed adds the IP addresses to the used store.
func (s *Subnet) AddUsed(list ...string) error {
	for _, value := range list {
		s.usedStore[value] = struct{}{}
	}
	return nil
}

// TotalUsedInt returns the number of used IP addresses in the subnet.
func (s *Subnet) TotalUsedInt() int {
	return len(s.usedStore)
}

// Total returns the number of IP addresses in the subnet.
func (s *Subnet) Total() *big.Int {
	total := big.NewInt(0)
	for _, r := range s.ranges {
		total.Add(total, big.NewInt(1))
		total.Add(total, new(big.Int).Sub(r.End, r.Start))
	}
	return total
}

// TotalInt returns the number of IP addresses in the subnet as an integer.
func (s *Subnet) TotalInt() int {
	totalInt64 := s.Total().Int64()
	if s.Total().Cmp(big.NewInt(math.MaxInt64)) > 0 {
		return math.MaxInt64
	}
	return int(totalInt64)
}

func (s *Subnet) addRange(raw string, start *big.Int, end *big.Int) error {
	if start == nil || end == nil {
		return fmt.Errorf("nil range: %s", raw)
	}
	// if start > end, start = end, end = start
	if start.Cmp(end) > 0 {
		start, end = end, start
	}
	a := Range{Raw: raw, Start: start, End: end}
	for _, b := range s.ranges {
		// a.start in b
		if a.Start.Cmp(b.Start) >= 0 && a.End.Cmp(b.End) <= 0 {
			return fmt.Errorf("range %s is already covered by %s", a.Raw, b.Raw)
		}
		// a.end in b
		if a.Start.Cmp(b.Start) <= 0 && a.End.Cmp(b.End) >= 0 {
			return fmt.Errorf("range %s covers %s", a.Raw, b.Raw)
		}
		// b.start in a
		if a.Start.Cmp(b.Start) >= 0 && a.Start.Cmp(b.End) <= 0 {
			return fmt.Errorf("range %s overlaps with %s", a.Raw, b.Raw)
		}
		// b.end in a
		if a.End.Cmp(b.Start) >= 0 && a.End.Cmp(b.End) <= 0 {
			return fmt.Errorf("range %s overlaps with %s", a.Raw, b.Raw)
		}
	}
	s.ranges = append(s.ranges, a)
	return nil
}

// parse parses the range string and returns the start and end IP addresses.
// case ipv4
// - "10.6.0.1"
// - "10.6.0.1-10.6.0.1"
// - "10.6.0.1/24"
// - ipv6
// - "fd00:db8::1"
// - "fd00:db8::1-fd00:db8::1"
// - "fd00:db8::1/64"
func (s *Subnet) parse(value string) (*big.Int, *big.Int, error) {
	parts := strings.Split(value, "-")
	if len(parts) > 2 {
		return nil, nil, fmt.Errorf("invalid range: %s", value)
	}
	if len(parts) == 1 {
		if strings.Contains(value, "/") {
			return nil, nil, fmt.Errorf("invalid range: %s", value)
		} else {
			res, err := s.toBigInt(value)
			return res, res, err
		}
	}
	start, err := s.toBigInt(parts[0])
	if err != nil {
		return nil, nil, err
	}
	end, err := s.toBigInt(parts[1])
	if err != nil {
		return nil, nil, err
	}
	return start, end, nil
}

// getBigInt converts the IP address to big.Int.
func (s *Subnet) toBigInt(value string) (*big.Int, error) {
	ip := net.ParseIP(value)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", value)
	}
	if !s.base.Contains(ip) {
		return nil, fmt.Errorf("not match base subnet: %s", value)
	}
	if s.isIPv4 {
		ipv4 := ip.To4()
		if ipv4 == nil {
			return nil, fmt.Errorf("not match base subnet IPv4 version: %s", value)
		}
		res := big.NewInt(0).SetBytes(ipv4)
		return res, nil
	} else {
		ipv6 := ip.To16()
		if ipv6 == nil {
			return nil, fmt.Errorf("not match base subnet IPv6 version: %s", value)
		}
		res := big.NewInt(0).SetBytes(ipv6)
		return res, nil
	}
}

func (s *Subnet) Ranges() []Range {
	return s.ranges
}

func (s *Subnet) IsOverlap(ranges []Range) ([]string, bool) {
	overlaps := make([]string, 0)
	for _, x := range s.ranges {
		for _, y := range ranges {
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

type Range struct {
	Raw        string
	Start, End *big.Int
}

func removeExcludeRange(base []Range, exclude []Range) []Range {
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
