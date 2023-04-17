// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip_test

import (
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
)

var _ = Describe("IP range", Label("ip_range_test"), func() {
	Describe("Test MergeIPRanges", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				ranges, err := spiderpoolip.MergeIPRanges(constant.InvalidIPVersion, []string{"172.18.40.10"})
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(ranges).To(BeEmpty())
			})

			It("inputs invalid IP ranges", func() {
				ranges, err := spiderpoolip.MergeIPRanges(constant.IPv4, constant.InvalidIPRanges)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(ranges).To(BeEmpty())
			})
		})

		It("merges IPv4 IP ranges", func() {
			ranges, err := spiderpoolip.MergeIPRanges(constant.IPv4,
				[]string{
					"172.18.40.10",
					"172.18.40.1-172.18.40.2",
					"172.18.40.2-172.18.40.3",
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ranges).To(Equal(
				[]string{
					"172.18.40.1-172.18.40.3",
					"172.18.40.10",
				},
			))
		})

		It("merges IPv6 IP ranges", func() {
			ranges, err := spiderpoolip.MergeIPRanges(constant.IPv6,
				[]string{
					"abcd:1234::a",
					"abcd:1234::1-abcd:1234::2",
					"abcd:1234::2-abcd:1234::3",
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ranges).To(Equal(
				[]string{
					"abcd:1234::1-abcd:1234::3",
					"abcd:1234::a",
				},
			))
		})
	})

	Describe("Test ParseIPRanges", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				ips, err := spiderpoolip.ParseIPRanges(constant.InvalidIPVersion, []string{"172.18.40.10"})
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(ips).To(BeEmpty())
			})

			It("inputs invalid IP ranges", func() {
				ips, err := spiderpoolip.ParseIPRanges(constant.IPv4, constant.InvalidIPRanges)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(ips).To(BeEmpty())
			})
		})

		It("parses IPv4 IP ranges", func() {
			ips, err := spiderpoolip.ParseIPRanges(constant.IPv4, []string{"172.18.40.10"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal([]net.IP{net.IPv4(172, 18, 40, 10)}))

			ips, err = spiderpoolip.ParseIPRanges(constant.IPv4,
				[]string{
					"172.18.40.10",
					"172.18.40.1-172.18.40.2",
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal(
				[]net.IP{
					net.IPv4(172, 18, 40, 10),
					net.IPv4(172, 18, 40, 1),
					net.IPv4(172, 18, 40, 2),
				},
			))
		})

		It("parses IPv6 IP ranges", func() {
			ips, err := spiderpoolip.ParseIPRanges(constant.IPv6, []string{"abcd:1234::a"})
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal([]net.IP{net.ParseIP("abcd:1234::a")}))

			ips, err = spiderpoolip.ParseIPRanges(constant.IPv6,
				[]string{
					"abcd:1234::a",
					"abcd:1234::1-abcd:1234::2",
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal(
				[]net.IP{
					net.ParseIP("abcd:1234::a"),
					net.ParseIP("abcd:1234::1"),
					net.ParseIP("abcd:1234::2"),
				},
			))
		})
	})

	Describe("Test ParseIPRange", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				ips, err := spiderpoolip.ParseIPRange(constant.InvalidIPVersion, "172.18.40.10")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(ips).To(BeEmpty())
			})

			It("inputs invalid IP ranges", func() {
				ips, err := spiderpoolip.ParseIPRange(constant.IPv4, constant.InvalidIPRange)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(ips).To(BeEmpty())
			})
		})

		It("parses IPv4 IP range", func() {
			ips, err := spiderpoolip.ParseIPRange(constant.IPv4, "172.18.40.10")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal([]net.IP{net.IPv4(172, 18, 40, 10)}))

			ips, err = spiderpoolip.ParseIPRange(constant.IPv4, "172.18.40.1-172.18.40.2")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal(
				[]net.IP{
					net.IPv4(172, 18, 40, 1),
					net.IPv4(172, 18, 40, 2),
				},
			))
		})

		It("parses IPv6 IP range", func() {
			ips, err := spiderpoolip.ParseIPRange(constant.IPv6, "abcd:1234::a")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal([]net.IP{net.ParseIP("abcd:1234::a")}))

			ips, err = spiderpoolip.ParseIPRange(constant.IPv6, "abcd:1234::1-abcd:1234::2")
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal(
				[]net.IP{
					net.ParseIP("abcd:1234::1"),
					net.ParseIP("abcd:1234::2"),
				},
			))
		})
	})

	Describe("Test ConvertIPsToIPRanges", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				ranges, err := spiderpoolip.ConvertIPsToIPRanges(constant.InvalidIPVersion, []net.IP{net.IPv4(172, 18, 40, 10)})
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(ranges).To(BeEmpty())
			})

			It("inputs unmatched IP addresses", func() {
				ranges, err := spiderpoolip.ConvertIPsToIPRanges(constant.IPv6, []net.IP{net.IPv4(172, 18, 40, 10)})
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIP))
				Expect(ranges).To(BeEmpty())
			})
		})

		It("converts IPv4 IP addresses to IP ranges", func() {
			ranges, err := spiderpoolip.ConvertIPsToIPRanges(constant.IPv4, []net.IP{net.IPv4(172, 18, 40, 10)})
			Expect(err).NotTo(HaveOccurred())
			Expect(ranges).To(Equal([]string{"172.18.40.10"}))

			ranges, err = spiderpoolip.ConvertIPsToIPRanges(constant.IPv4,
				[]net.IP{
					net.IPv4(172, 18, 40, 10),
					net.IPv4(172, 18, 40, 1),
					net.IPv4(172, 18, 40, 2),
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ranges).To(Equal(
				[]string{
					"172.18.40.1-172.18.40.2",
					"172.18.40.10",
				},
			))
		})

		It("converts IPv6 IP addresses to IP ranges", func() {
			ranges, err := spiderpoolip.ConvertIPsToIPRanges(constant.IPv6, []net.IP{net.ParseIP("abcd:1234::a")})
			Expect(err).NotTo(HaveOccurred())
			Expect(ranges).To(Equal([]string{"abcd:1234::a"}))

			ranges, err = spiderpoolip.ConvertIPsToIPRanges(constant.IPv6,
				[]net.IP{
					net.ParseIP("abcd:1234::a"),
					net.ParseIP("abcd:1234::1"),
					net.ParseIP("abcd:1234::2"),
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ranges).To(Equal(
				[]string{
					"abcd:1234::1-abcd:1234::2",
					"abcd:1234::a",
				},
			))
		})
	})

	Describe("Test ContainsIPRange", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				contains, err := spiderpoolip.ContainsIPRange(constant.InvalidIPVersion, "172.18.40.0/24", "172.18.40.10")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(contains).To(BeFalse())
			})

			It("inputs invalid subnet", func() {
				contains, err := spiderpoolip.ContainsIPRange(constant.IPv4, constant.InvalidCIDR, "172.18.40.10")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidCIDRFormat))
				Expect(contains).To(BeFalse())
			})

			It("inputs invalid IP range", func() {
				contains, err := spiderpoolip.ContainsIPRange(constant.IPv4, "172.18.40.0/24", constant.InvalidIPRange)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(contains).To(BeFalse())
			})
		})

		When("IPv4", func() {
			It("tests that a subnet contains the IP range", func() {
				contains, err := spiderpoolip.ContainsIPRange(constant.IPv4, "172.18.40.0/24", "172.18.40.10")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeTrue())

				contains, err = spiderpoolip.ContainsIPRange(constant.IPv4, "172.18.40.0/24", "172.18.40.1-172.18.40.2")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeTrue())
			})

			It("test that a subnet does not contain the IP range", func() {
				contains, err := spiderpoolip.ContainsIPRange(constant.IPv4, "172.18.40.0/24", "172.18.40.254-172.18.41.1")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeFalse())
			})
		})

		When("IPv6", func() {
			It("tests that a subnet contains the IP range", func() {
				contains, err := spiderpoolip.ContainsIPRange(constant.IPv6, "abcd:1234::/120", "abcd:1234::a")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeTrue())

				contains, err = spiderpoolip.ContainsIPRange(constant.IPv6, "abcd:1234::/120", "abcd:1234::1-abcd:1234::2")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeTrue())
			})

			It("test that a subnet does not contain the IP range", func() {
				contains, err := spiderpoolip.ContainsIPRange(constant.IPv6, "abcd:1234::/120", "abcd:1234::fd-abcd:1234::101")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeFalse())
			})
		})
	})

	Describe("Test IPRangeContainsIP", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				contains, err := spiderpoolip.IPRangeContainsIP(constant.InvalidIPVersion, "172.18.40.1-172.18.40.2", "172.18.40.1")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(contains).To(BeFalse())
			})

			It("inputs invalid IP range", func() {
				contains, err := spiderpoolip.IPRangeContainsIP(constant.IPv4, constant.InvalidIPRange, "172.18.40.1")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(contains).To(BeFalse())
			})

			It("inputs invalid IP", func() {
				contains, err := spiderpoolip.IPRangeContainsIP(constant.IPv4, "172.18.40.1-172.18.40.2", constant.InvalidIP)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPFormat))
				Expect(contains).To(BeFalse())
			})
		})

		When("IPv4", func() {
			It("tests that a IP range contains the IP", func() {
				contains, err := spiderpoolip.IPRangeContainsIP(constant.IPv4, "172.18.40.1", "172.18.40.1")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeTrue())

				contains, err = spiderpoolip.IPRangeContainsIP(constant.IPv4, "172.18.40.1-172.18.40.2", "172.18.40.1")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeTrue())
			})

			It("test that a IP range does not contain the IP", func() {
				contains, err := spiderpoolip.IPRangeContainsIP(constant.IPv4, "172.18.40.1-172.18.40.2", "172.18.40.10")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeFalse())
			})
		})

		When("IPv6", func() {
			It("tests that a IP range contains the IP", func() {
				contains, err := spiderpoolip.IPRangeContainsIP(constant.IPv6, "abcd:1234::1", "abcd:1234::1")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeTrue())

				contains, err = spiderpoolip.IPRangeContainsIP(constant.IPv6, "abcd:1234::1-abcd:1234::2", "abcd:1234::1")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeTrue())
			})

			It("test that a IP range does not contain the IP", func() {
				contains, err := spiderpoolip.IPRangeContainsIP(constant.IPv6, "abcd:1234::1-abcd:1234::2", "abcd:1234::a")
				Expect(err).NotTo(HaveOccurred())
				Expect(contains).To(BeFalse())
			})
		})
	})

	Describe("Test IsIPRangeOverlap", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				overlap, err := spiderpoolip.IsIPRangeOverlap(constant.InvalidIPVersion, "172.18.40.1-172.18.40.2", "172.18.40.2-172.18.40.3")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(overlap).To(BeFalse())
			})

			It("inputs invalid IP range", func() {
				overlap, err := spiderpoolip.IsIPRangeOverlap(constant.IPv4, constant.InvalidIPRange, "172.18.40.2-172.18.40.3")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(overlap).To(BeFalse())

				overlap, err = spiderpoolip.IsIPRangeOverlap(constant.IPv4, "172.18.40.1-172.18.40.2", constant.InvalidIPRange)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(overlap).To(BeFalse())
			})
		})

		When("IPv4", func() {
			It("tests that two IP ranges overlap", func() {
				overlap, err := spiderpoolip.IsIPRangeOverlap(constant.IPv4, "172.18.40.1-172.18.40.2", "172.18.40.2-172.18.40.3")
				Expect(err).NotTo(HaveOccurred())
				Expect(overlap).To(BeTrue())
			})

			It("tests that two IP ranges do not overlap", func() {
				overlap, err := spiderpoolip.IsIPRangeOverlap(constant.IPv4, "172.18.40.1-172.18.40.2", "172.18.40.3-172.18.40.4")
				Expect(err).NotTo(HaveOccurred())
				Expect(overlap).To(BeFalse())
			})
		})

		When("IPv6", func() {
			It("tests that two IP ranges overlap", func() {
				overlap, err := spiderpoolip.IsIPRangeOverlap(constant.IPv6, "abcd:1234::1-abcd:1234::2", "abcd:1234::2-abcd:1234::3")
				Expect(err).NotTo(HaveOccurred())
				Expect(overlap).To(BeTrue())
			})

			It("tests that two IP ranges do not overlap", func() {
				overlap, err := spiderpoolip.IsIPRangeOverlap(constant.IPv6, "abcd:1234::1-abcd:1234::2", "abcd:1234::3-abcd:1234::4")
				Expect(err).NotTo(HaveOccurred())
				Expect(overlap).To(BeFalse())
			})
		})
	})

	Describe("Test IsIPRange", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				err := spiderpoolip.IsIPRange(constant.InvalidIPVersion, "172.18.40.1-172.18.40.2")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
			})

			It("inputs invalid IP range", func() {
				err := spiderpoolip.IsIPRange(constant.IPv4, constant.InvalidIPRange)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
			})
		})

		It("is an IPv4 IP range", func() {
			Expect(spiderpoolip.IsIPRange(constant.IPv4, "172.18.40.10")).To(Succeed())
			Expect(spiderpoolip.IsIPRange(constant.IPv4, "172.18.40.1-172.18.40.2")).To(Succeed())
		})

		It("is an IPv6 IP range", func() {
			Expect(spiderpoolip.IsIPRange(constant.IPv6, "abcd:1234::a")).To(Succeed())
			Expect(spiderpoolip.IsIPRange(constant.IPv6, "abcd:1234::1-abcd:1234::2")).To(Succeed())
		})
	})

	Describe("Test IsIPv4IPRange", func() {
		It("tests whether it is an IPv4 IP range", func() {
			Expect(spiderpoolip.IsIPv4IPRange(constant.InvalidIPRange)).To(BeFalse())
			Expect(spiderpoolip.IsIPv4IPRange("172.18.40.1-invalid IP range")).To(BeFalse())
			Expect(spiderpoolip.IsIPv4IPRange("invalid IP range-172.18.40.2")).To(BeFalse())
			Expect(spiderpoolip.IsIPv4IPRange("172.18.40.2-172.18.40.1")).To(BeFalse())
			Expect(spiderpoolip.IsIPv4IPRange("172.18.40.1-172.18.40.2-172.18.40.3")).To(BeFalse())

			Expect(spiderpoolip.IsIPv4IPRange("172.18.40.10")).To(BeTrue())
			Expect(spiderpoolip.IsIPv4IPRange("172.18.40.1-172.18.40.2")).To(BeTrue())
		})
	})

	Describe("Test IsIPv6IPRange", func() {
		It("tests whether it is an IPv6 IP range", func() {
			Expect(spiderpoolip.IsIPv6IPRange(constant.InvalidIPRange)).To(BeFalse())
			Expect(spiderpoolip.IsIPv6IPRange("abcd:1234::1-invalid IP range")).To(BeFalse())
			Expect(spiderpoolip.IsIPv6IPRange("invalid IP range-abcd:1234::2")).To(BeFalse())
			Expect(spiderpoolip.IsIPv6IPRange("abcd:1234::2-abcd:1234::1")).To(BeFalse())
			Expect(spiderpoolip.IsIPv6IPRange("abcd:1234::1-abcd:1234::2-abcd:1234::3")).To(BeFalse())

			Expect(spiderpoolip.IsIPv6IPRange("abcd:1234::a")).To(BeTrue())
			Expect(spiderpoolip.IsIPv6IPRange("abcd:1234::1-abcd:1234::2")).To(BeTrue())
		})
	})
})
