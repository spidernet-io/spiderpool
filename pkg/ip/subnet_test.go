// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("NewCIDR", Label("subnet_test"), func() {
	testCases := []struct {
		name            string
		base            string
		ranges          []string
		exclude         []string
		expErr          bool
		expTotal        int
		expRanges       []string
		overlapIPRanges []string
		expIsOverlap    bool
	}{
		{
			name:     "empty ranges and exclude",
			base:     "10.6.0.0/24",
			ranges:   []string{},
			exclude:  []string{},
			expErr:   false,
			expTotal: 0,
		},
		{
			name:     "ipv4 subnet contain range",
			base:     "10.6.0.0/24",
			ranges:   []string{"10.6.0.1-10.6.0.2"},
			exclude:  nil,
			expErr:   false,
			expTotal: 2,
			expRanges: []string{
				"10.6.0.1-10.6.0.2",
			},
		},
		{
			name: "ipv4 subnet contain multiple ranges",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-10.6.0.2",
				"10.6.0.4-10.6.0.6",
			},
			exclude:  nil,
			expErr:   false,
			expTotal: 5,
			expRanges: []string{
				"10.6.0.1-10.6.0.2",
				"10.6.0.4-10.6.0.6",
			},
		},
		{
			name: "fully overlapping ranges",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-10.6.0.10",
				"10.6.0.1-10.6.0.10",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name:      "ipv4 subnet contain ip",
			base:      "10.6.0.0/24",
			ranges:    []string{"10.6.0.1"},
			exclude:   nil,
			expErr:    false,
			expTotal:  1,
			expRanges: []string{"10.6.0.1"},
		},
		{
			name: "ipv4 subnet not contain ip",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.5.0.1",
			},
			exclude:  nil,
			expErr:   true,
			expTotal: 0,
		},
		{
			name: "ipv4 subnet not contain range",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.5.0.1-10.6.0.10",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "ipv4 subnet range overlap range case1",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-10.6.0.10",
				"10.6.0.10-10.6.0.20",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "ipv4 subnet range overlap range case2",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-10.6.0.10",
				"10.6.0.3-10.6.0.5",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "ipv4 subnet range overlap ip",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-10.6.0.10",
				"10.6.0.3",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "ipv4 subnet invalid range",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "ipv4 subnet invalid range",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-fd:00::1",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "ipv4 subnet invalid range",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-fd:00::1",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "invalid IP address format",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-10.6.0.300",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "invalid CIDR format",
			base: "10.6.0.0/33",
			ranges: []string{
				"10.6.0.1-10.6.0.10",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name:      "case signal exclude",
			base:      "10.6.0.0/24",
			ranges:    []string{"10.6.0.1-10.6.0.10"},
			exclude:   []string{"10.6.0.1-10.6.0.2"},
			expErr:    false,
			expTotal:  8,
			expRanges: []string{"10.6.0.3-10.6.0.10"},
		},
		{
			name: "case multi exclude",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-10.6.0.10",
			},
			exclude: []string{
				"10.6.0.1-10.6.0.2",
				"10.6.0.8-10.6.0.10",
			},
			expErr:   false,
			expTotal: 5,
			expRanges: []string{
				"10.6.0.3-10.6.0.7",
			},
		},
		{
			name: "case multi exclude",
			base: "10.6.0.0/24",
			ranges: []string{
				"10.6.0.1-10.6.0.10",
				"10.6.0.11-10.6.0.20",
				"10.6.0.21-10.6.0.30",
			},
			exclude: []string{
				"10.6.0.1-10.6.0.2",
				"10.6.0.8-10.6.0.22",
			},
			expErr:   false,
			expTotal: 13,
			expRanges: []string{
				"10.6.0.3-10.6.0.7",
				"10.6.0.23-10.6.0.30",
			},
		},

		{
			name:     "ipv6 subnet contain range",
			base:     "fd00::/64",
			ranges:   []string{"fd00::1-fd00::2"},
			exclude:  nil,
			expErr:   false,
			expTotal: 2,
			expRanges: []string{
				"fd00::1-fd00::2",
			},
		},

		{
			name: "ipv6 subnet contain multiple ranges",
			base: "fd00::/64",
			ranges: []string{
				"fd00::1-fd00::2",
				"fd00::4-fd00::6",
			},
			exclude:  nil,
			expErr:   false,
			expTotal: 5,
			expRanges: []string{
				"fd00::1-fd00::2",
				"fd00::4-fd00::6",
			},
		},

		{
			name: "ipv6 subnet contain multiple ranges",
			base: "fd00::/64",
			ranges: []string{
				"fd00::1-fd00::2",
				"fd00::4-fd00::6",
			},
			exclude:  nil,
			expErr:   false,
			expTotal: 5,
			expRanges: []string{
				"fd00::1-fd00::2",
				"fd00::4-fd00::6",
			},
		},
		{
			name: "ipv6 subnet invalid range",
			base: "fd00::/64",
			ranges: []string{
				"fd00::1-",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "ipv6 subnet invalid range",
			base: "fd00::/64",
			ranges: []string{
				"fd00::1-10.6.0.1",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name: "invalid IPv6 address format",
			base: "fd00::/64",
			ranges: []string{
				"fd00::1-fd00::zzzz",
			},
			exclude: nil,
			expErr:  true,
		},
		{
			name:      "case signal exclude",
			base:      "fd00::/64",
			ranges:    []string{"fd00::1-fd00::10"},
			exclude:   []string{"fd00::1-fd00::2"},
			expErr:    false,
			expTotal:  14,
			expRanges: []string{"fd00::3-fd00::10"},
		},
		{
			name: "case multi exclude",
			base: "fd00::/64",
			ranges: []string{
				"fd00::1-fd00::10",
				"fd00::11-fd00::20",
				"fd00::21-fd00::30",
			},
			exclude: []string{
				"fd00::1-fd00::2",
				"fd00::8-fd00::22",
			},
			expErr:   false,
			expTotal: 19,
			expRanges: []string{
				"fd00::3-fd00::7",
				"fd00::23-fd00::30",
			},
		},
		{
			name: "case for overlap ranges",
			base: "fd12:3456:789a:baba::/64",
			ranges: []string{
				"fd12:3456:789a:baba::1-fd12:3456:789a:baba:ffff:ffff:ffff:ffff",
			},
			exclude: []string{
				"fd12:3456:789a:baba::1-fd12:3456:789a:baba::10",
			},
			expErr:   false,
			expTotal: 9223372036854775807,
			expRanges: []string{
				"fd12:3456:789a:baba::11-fd12:3456:789a:baba:ffff:ffff:ffff:ffff",
			},
			overlapIPRanges: []string{"fd12:3456:789a:baba::11-fd12:3456:789a:baba::12"},
			expIsOverlap:    true,
		},
	}

	for _, c := range testCases {
		c := c // capture range variable
		It(c.name, func() {
			subnet, err := NewCIDR(c.base, c.ranges, c.exclude)
			if c.expErr {
				Expect(err).To(HaveOccurred())
				return
			} else {
				Expect(err).NotTo(HaveOccurred())
			}

			Expect(subnet.TotalIPInt()).To(Equal(c.expTotal))

			var expRanges []Range
			for _, raw := range c.expRanges {
				start, end, err := subnet.parse(raw)
				Expect(err).NotTo(HaveOccurred())
				expRanges = append(expRanges, Range{Raw: raw, Start: start, End: end})
			}

			Expect(compareRanges(subnet.IPRange(), expRanges)).To(BeTrue())

			var overlapIPRanges []Range
			for _, raw := range c.overlapIPRanges {
				start, end, err := subnet.parse(raw)
				Expect(err).NotTo(HaveOccurred(), "case - %v, error parsing range %v: %v", c.name, raw, err)
				overlapIPRanges = append(overlapIPRanges, Range{Raw: raw, Start: start, End: end})
			}

			_, isOverlap := subnet.IsOverlapIPRanges(overlapIPRanges)
			Expect(isOverlap).To(Equal(c.expIsOverlap), "case - %v, expected overlap %v but got %v", c.name, c.expIsOverlap, isOverlap)
		})
	}
})

func compareRanges(a, b []Range) bool {
	if len(a) != len(b) {
		return false
	}

	mapA := make(map[string]Range)
	mapB := make(map[string]Range)

	for _, r := range a {
		mapA[r.Start.String()+"-"+r.End.String()] = r
	}
	for _, r := range b {
		mapB[r.Start.String()+"-"+r.End.String()] = r
	}

	for key, rangeA := range mapA {
		rangeB, exists := mapB[key]
		if !exists || rangeA.Start.Cmp(rangeB.Start) != 0 || rangeA.End.Cmp(rangeB.End) != 0 {
			return false
		}
	}

	return true
}
