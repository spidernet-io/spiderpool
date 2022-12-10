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

var _ = Describe("IP utils", Label("ip_utils_test"), func() {
	Describe("Test AssembleTotalIPs", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				ips, err := spiderpoolip.AssembleTotalIPs(constant.InvalidIPVersion, []string{"172.18.40.1-172.18.40.2"}, []string{"172.18.40.2"})
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(ips).To(BeEmpty())
			})

			It("inputs invalid IP ranges", func() {
				ips, err := spiderpoolip.AssembleTotalIPs(constant.IPv4, constant.InvalidIPRanges, []string{"172.18.40.2"})
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(ips).To(BeEmpty())
			})

			It("inputs invalid excluded IP ranges", func() {
				ips, err := spiderpoolip.AssembleTotalIPs(constant.IPv4, []string{"172.18.40.1-172.18.40.2"}, constant.InvalidIPRanges)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(ips).To(BeEmpty())
			})
		})

		It("assembles all valid IPv4 IP addresses", func() {
			ips, err := spiderpoolip.AssembleTotalIPs(constant.IPv4,
				[]string{
					"172.18.40.10",
					"172.18.40.1-172.18.40.2",
				},
				[]string{
					"172.18.40.10",
					"172.18.40.2-172.18.40.3",
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal([]net.IP{net.IPv4(172, 18, 40, 1)}))
		})

		It("assembles all valid IPv6 IP addresses", func() {
			ips, err := spiderpoolip.AssembleTotalIPs(constant.IPv6,
				[]string{
					"abcd:1234::a",
					"abcd:1234::1-abcd:1234::2",
				},
				[]string{
					"abcd:1234::a",
					"abcd:1234::2-abcd:1234::3",
				},
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal([]net.IP{net.ParseIP("abcd:1234::1")}))
		})
	})
})
