// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking_test

import (
	"net"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"

	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
)

var _ = Describe("IP family detection", Label("networking_ip_test"), func() {
	Describe("Test GetIPFamilyByResult", func() {
		When("Verifying", func() {
			It("inputs a result with no interfaces", func() {
				family, err := networking.GetIPFamilyByResult(&current.Result{})
				Expect(err).To(HaveOccurred())
				Expect(family).To(Equal(-1))
			})

			It("inputs a result with interfaces but no IPs", func() {
				r := &current.Result{Interfaces: []*current.Interface{{Name: "eth0"}}}
				family, err := networking.GetIPFamilyByResult(r)
				Expect(err).To(HaveOccurred())
				Expect(family).To(Equal(-1))
			})
		})

		It("returns FAMILY_V4 for v4-only IPs", func() {
			family, err := networking.GetIPFamilyByResult(makePrevResult("192.0.2.6/24"))
			Expect(err).NotTo(HaveOccurred())
			Expect(family).To(Equal(netlink.FAMILY_V4))
		})

		It("returns FAMILY_V6 for v6-only IPs", func() {
			family, err := networking.GetIPFamilyByResult(makePrevResult("2001:db8:abcd::6/64"))
			Expect(err).NotTo(HaveOccurred())
			Expect(family).To(Equal(netlink.FAMILY_V6))
		})

		It("returns FAMILY_ALL for dual-stack IPs", func() {
			family, err := networking.GetIPFamilyByResult(
				makePrevResult("192.0.2.6/24", "2001:db8:abcd::6/64"),
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(family).To(Equal(netlink.FAMILY_ALL))
		})
	})

	// Behavioral coverage for GetIPFamilyByIface (v4-only, v6-only, dual,
	// missing-iface, no-addrs) lives in the integration suite, which exercises
	// the real netlink path. The unit tests here only cover input validation.
	Describe("Test GetIPFamilyByIface", func() {
		When("Verifying inputs", func() {
			It("errors when netns is nil", func() {
				family, err := networking.GetIPFamilyByIface(nil, "eth0")
				Expect(err).To(HaveOccurred())
				Expect(family).To(Equal(-1))
			})

			It("errors when ifName is empty", func() {
				family, err := networking.GetIPFamilyByIface(sentinelNetns{}, "")
				Expect(err).To(HaveOccurred())
				Expect(family).To(Equal(-1))
			})
		})
	})
})

func makePrevResult(cidrs ...string) *current.Result {
	r := &current.Result{Interfaces: []*current.Interface{{Name: "eth0"}}}
	for _, c := range cidrs {
		ip, ipnet, err := net.ParseCIDR(c)
		Expect(err).NotTo(HaveOccurred())
		r.IPs = append(r.IPs, &current.IPConfig{
			Address: net.IPNet{IP: ip, Mask: ipnet.Mask},
		})
	}
	return r
}

// sentinelNetns is a non-nil ns.NetNS placeholder for input-validation tests
// that never actually enter the netns.
type sentinelNetns struct{}

func (sentinelNetns) Do(_ func(ns.NetNS) error) error { return nil }
func (sentinelNetns) Set() error                      { return nil }
func (sentinelNetns) Path() string                    { return "" }
func (sentinelNetns) Fd() uintptr                     { return 0 }
func (sentinelNetns) Close() error                    { return nil }
