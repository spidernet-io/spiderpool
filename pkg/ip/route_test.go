// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ip_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
)

var _ = Describe("Route", Label("route_test"), func() {
	Describe("Test IsRoute", func() {
		When("Verifying", func() {
			It("inputs invalid IP version", func() {
				err := spiderpoolip.IsRoute(constant.InvalidIPVersion, "192.168.40.0/24", "172.18.40.1")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
			})

			It("inputs invalid routing destination", func() {
				err := spiderpoolip.IsRoute(constant.IPv4, constant.InvalidDst, "172.18.40.1")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidRouteFormat))
			})

			It("inputs invalid routing gateway", func() {
				err := spiderpoolip.IsRoute(constant.IPv4, "192.168.40.0/24", constant.InvalidGateway)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidRouteFormat))
			})
		})

		It("is an IPv4 route", func() {
			err := spiderpoolip.IsRoute(constant.IPv4, "192.168.40.0/24", "172.18.40.1")
			Expect(err).NotTo(HaveOccurred())
		})

		It("is an IPv6 route", func() {
			err := spiderpoolip.IsRoute(constant.IPv6, "fd00:40::/120", "abcd:1234::1")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Test IsRouteWithoutIPVersion", func() {
		When("Verifying", func() {
			It("inputs invalid routing destination", func() {
				err := spiderpoolip.IsRouteWithoutIPVersion(constant.InvalidDst, "172.18.40.1")
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidRouteFormat))
			})

			It("inputs invalid routing gateway", func() {
				err := spiderpoolip.IsRouteWithoutIPVersion("192.168.40.0/24", constant.InvalidGateway)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidRouteFormat))
			})
		})

		It("is an route", func() {
			err := spiderpoolip.IsRouteWithoutIPVersion("192.168.40.0/24", "172.18.40.1")
			Expect(err).NotTo(HaveOccurred())

			err = spiderpoolip.IsRouteWithoutIPVersion("fd00:40::/120", "abcd:1234::1")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Test IsIPv4Route", func() {
		It("tests whether it is an IPv4 route", func() {
			Expect(spiderpoolip.IsIPv4Route(constant.InvalidDst, "172.18.40.1")).To(BeFalse())
			Expect(spiderpoolip.IsIPv4Route("192.168.40.0/24", constant.InvalidGateway)).To(BeFalse())
			Expect(spiderpoolip.IsIPv4Route("192.168.40.0/24", "172.18.40.1")).To(BeTrue())
		})
	})

	Describe("Test IsIPv6Route", func() {
		It("tests whether it is an IPv6 route", func() {
			Expect(spiderpoolip.IsIPv6Route(constant.InvalidDst, "abcd:1234::1")).To(BeFalse())
			Expect(spiderpoolip.IsIPv6Route("fd00:40::/120", constant.InvalidGateway)).To(BeFalse())
			Expect(spiderpoolip.IsIPv6Route("fd00:40::/120", "abcd:1234::1")).To(BeTrue())
		})
	})
})
