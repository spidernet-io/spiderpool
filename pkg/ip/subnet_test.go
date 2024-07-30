// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Subnet", Label("subnet_test"), func() {
	Describe("NewSubnet", func() {
		testCases := []struct {
			name     string
			base     string
			ranges   []string
			exclude  []string
			expErr   bool
			expTotal int
		}{
			{
				name:     "ipv4 subnet contain range",
				base:     "10.6.0.0/24",
				ranges:   []string{"10.6.0.1-10.6.0.2"},
				exclude:  nil,
				expErr:   false,
				expTotal: 2,
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
			},
			{
				name: "ipv4 subnet contain ip",
				base: "10.6.0.0/24",
				ranges: []string{
					"10.6.0.1",
				},
				exclude:  nil,
				expErr:   false,
				expTotal: 1,
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
				name: "case single exclude",
				base: "10.6.0.0/24",
				ranges: []string{
					"10.6.0.1-10.6.0.10",
				},
				exclude: []string{
					"10.6.0.1-10.6.0.2",
				},
				expErr:   false,
				expTotal: 8,
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
			},
		}

		for _, c := range testCases {
			c := c
			It(c.name, func() {
				subnet, err := NewSubnet(c.base, c.ranges, c.exclude)
				if c.expErr {
					Expect(err).To(HaveOccurred())
				} else {
					Expect(err).NotTo(HaveOccurred())
					Expect(subnet.TotalInt()).To(Equal(c.expTotal))
				}
			})
		}
	})
})
