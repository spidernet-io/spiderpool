// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking_test

import (
	"errors"
	"net"

	"github.com/agiledragon/gomonkey/v2"
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

	Describe("Test GetIPFamilyByResultWithIface", func() {
		var (
			v4Result    *current.Result
			v6Result    *current.Result
			dualResult  *current.Result
			emptyResult *current.Result
			fakeNetns   ns.NetNS
			slaacV6     netlink.Addr
		)

		BeforeEach(func() {
			v4Result = makePrevResult("192.0.2.6/24")
			v6Result = makePrevResult("2001:db8:abcd::6/64")
			dualResult = makePrevResult("192.0.2.6/24", "2001:db8:abcd::6/64")
			emptyResult = &current.Result{Interfaces: []*current.Interface{{Name: "eth0"}}}
			fakeNetns = sentinelNetns{}
			slaacV6 = mustParseAddr("2001:db8:abcd:0:0a:bcff:fe00:0001/64")
		})

		When("PrevResult already has v6", func() {
			It("returns FAMILY_V6 without scanning the iface", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return nil, errors.New("must not be called")
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(v6Result, fakeNetns, "eth0")
				Expect(err).NotTo(HaveOccurred())
				Expect(family).To(Equal(netlink.FAMILY_V6))
			})

			It("returns FAMILY_ALL without scanning the iface", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return nil, errors.New("must not be called")
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(dualResult, fakeNetns, "eth0")
				Expect(err).NotTo(HaveOccurred())
				Expect(family).To(Equal(netlink.FAMILY_ALL))
			})
		})

		When("PrevResult has v4 only", func() {
			It("upgrades to FAMILY_ALL when the iface has SLAAC v6", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return []netlink.Addr{slaacV6}, nil
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(v4Result, fakeNetns, "eth0")
				Expect(err).NotTo(HaveOccurred())
				Expect(family).To(Equal(netlink.FAMILY_ALL))
			})

			It("keeps FAMILY_V4 when the iface has no v6", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return nil, nil
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(v4Result, fakeNetns, "eth0")
				Expect(err).NotTo(HaveOccurred())
				Expect(family).To(Equal(netlink.FAMILY_V4))
			})

			It("keeps FAMILY_V4 when the iface scan errors", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return nil, errors.New("link not found")
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(v4Result, fakeNetns, "eth0")
				Expect(err).NotTo(HaveOccurred())
				Expect(family).To(Equal(netlink.FAMILY_V4))
			})
		})

		When("PrevResult has no IPs", func() {
			It("returns FAMILY_V6 when the iface has SLAAC v6", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return []netlink.Addr{slaacV6}, nil
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(emptyResult, fakeNetns, "eth0")
				Expect(err).NotTo(HaveOccurred())
				Expect(family).To(Equal(netlink.FAMILY_V6))
			})

			It("propagates the parse error when the iface also has no v6", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return nil, nil
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(emptyResult, fakeNetns, "eth0")
				Expect(err).To(HaveOccurred())
				Expect(family).To(Equal(-1))
			})

			It("wraps both errors when the iface scan also fails", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return nil, errors.New("link not found")
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(emptyResult, fakeNetns, "eth0")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("link not found"))
				Expect(family).To(Equal(-1))
			})
		})

		When("the SLAAC fallback is unavailable", func() {
			It("skips the iface scan when netns is nil", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return nil, errors.New("must not be called")
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(v4Result, nil, "eth0")
				Expect(err).NotTo(HaveOccurred())
				Expect(family).To(Equal(netlink.FAMILY_V4))
			})

			It("skips the iface scan when ifName is empty", func() {
				patches := gomonkey.ApplyFunc(networking.IPAddressByName,
					func(_ ns.NetNS, _ string, _ int) ([]netlink.Addr, error) {
						return nil, errors.New("must not be called")
					})
				defer patches.Reset()

				family, err := networking.GetIPFamilyByResultWithIface(v4Result, fakeNetns, "")
				Expect(err).NotTo(HaveOccurred())
				Expect(family).To(Equal(netlink.FAMILY_V4))
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

func mustParseAddr(s string) netlink.Addr {
	a, err := netlink.ParseAddr(s)
	Expect(err).NotTo(HaveOccurred())
	return *a
}

// sentinelNetns is a stand-in for ns.NetNS used only as a non-nil pointer in
// tests; its methods are never called because IPAddressByName is mocked.
type sentinelNetns struct{}

func (sentinelNetns) Do(_ func(ns.NetNS) error) error { return nil }
func (sentinelNetns) Set() error                      { return nil }
func (sentinelNetns) Path() string                    { return "" }
func (sentinelNetns) Fd() uintptr                     { return 0 }
func (sentinelNetns) Close() error                    { return nil }
