// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package integration_test

// Real-netns coverage for GetIPFamilyByResultWithIface and the getAdders
// filter. Uses the same testutils.NewNS() + netlink.LinkAdd pattern as
// cmd/spiderpool/cmd/command_test.go; runs as part of `make unittest-tests`.
// Lives in its own ginkgo suite (separate test binary) to keep
// testutils.NewNS()'s OS-thread lifecycle from interacting with the
// gomonkey machine-code patches in the sibling unit tests.

import (
	"net"
	"time"

	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
)

var _ = Describe("IP family detection — real netns", Label("networking_realnetns_test"), func() {
	var testNetns ns.NetNS

	BeforeEach(func() {
		var err error
		testNetns, err = testutils.NewNS()
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			_ = testNetns.Close()
			_ = testutils.UnmountNS(testNetns)
		})
	})

	setupDummyIface := func(v4CIDRs, v6CIDRs []string) {
		err := testNetns.Do(func(_ ns.NetNS) error {
			dummy := &netlink.Dummy{LinkAttrs: netlink.LinkAttrs{Name: "eth0"}}
			if err := netlink.LinkAdd(dummy); err != nil {
				return err
			}
			link, err := netlink.LinkByName("eth0")
			if err != nil {
				return err
			}
			if err := netlink.LinkSetUp(link); err != nil {
				return err
			}
			for _, cidr := range v4CIDRs {
				addr, perr := netlink.ParseAddr(cidr)
				if perr != nil {
					return perr
				}
				if err := netlink.AddrAdd(link, addr); err != nil {
					return err
				}
			}
			for _, cidr := range v6CIDRs {
				addr, perr := netlink.ParseAddr(cidr)
				if perr != nil {
					return perr
				}
				// Dummy ifaces have IFF_NOARP so DAD is skipped anyway; NODAD
				// here is belt-and-suspenders so the helper is portable.
				addr.Flags |= unix.IFA_F_NODAD
				if err := netlink.AddrAdd(link, addr); err != nil {
					return err
				}
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	}

	makePrev := func(cidrs ...string) *current.Result {
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

	DescribeTable(
		"GetIPFamilyByResultWithIface against a real interface",
		func(prevCIDRs, ifaceV4, ifaceV6 []string, wantFamily int, wantErr bool) {
			setupDummyIface(ifaceV4, ifaceV6)

			gotFamily, err := networking.GetIPFamilyByResultWithIface(
				makePrev(prevCIDRs...), testNetns, "eth0",
			)
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(gotFamily).To(Equal(wantFamily))
		},
		Entry("dual-stack: v4 IPAM + kernel SLAAC v6 → FAMILY_ALL (the bug scenario)",
			[]string{"192.0.2.6/24"},
			[]string{"192.0.2.6/24"}, []string{"2001:db8:abcd:0:0a:bcff:fe00:0001/64"},
			netlink.FAMILY_ALL, false),
		Entry("v4-only IPAM, no v6 on iface → FAMILY_V4 unchanged",
			[]string{"192.0.2.6/24"},
			[]string{"192.0.2.6/24"}, nil,
			netlink.FAMILY_V4, false),
		Entry("v4 IPAM + only link-local v6 → FAMILY_V4 (LL filtered)",
			[]string{"192.0.2.6/24"},
			[]string{"192.0.2.6/24"}, nil,
			netlink.FAMILY_V4, false),
		Entry("dual-stack PrevResult unchanged (no scan needed)",
			[]string{"192.0.2.6/24", "2001:db8:abcd::6/64"},
			[]string{"192.0.2.6/24"}, []string{"2001:db8:abcd::6/64"},
			netlink.FAMILY_ALL, false),
		Entry("SLAAC-only (no IPAM) → FAMILY_V6 (pre-fix this errored)",
			nil,
			nil, []string{"2001:db8:abcd:0:0a:bcff:fe00:0001/64"},
			netlink.FAMILY_V6, false),
		Entry("no IPs anywhere → original parse error propagates",
			nil,
			nil, nil,
			-1, true),
	)

	Describe("getAdders filter — IFA_F_TENTATIVE / IFA_F_DADFAILED", func() {
		It("ignores a SLAAC v6 until DAD completes (RFC 4862 §5.4)", func() {
			// Dummy ifaces skip DAD entirely (IFF_NOARP), so use a veth pair —
			// the kernel runs real DAD across the pair and marks the address
			// IFA_F_TENTATIVE until DAD succeeds (~1-2s in steady state).
			err := testNetns.Do(func(_ ns.NetNS) error {
				veth := &netlink.Veth{
					LinkAttrs: netlink.LinkAttrs{Name: "eth0"},
					PeerName:  "veth-peer",
				}
				if err := netlink.LinkAdd(veth); err != nil {
					return err
				}
				for _, name := range []string{"eth0", "veth-peer"} {
					link, err := netlink.LinkByName(name)
					if err != nil {
						return err
					}
					if err := netlink.LinkSetUp(link); err != nil {
						return err
					}
				}
				link, _ := netlink.LinkByName("eth0")
				v4, _ := netlink.ParseAddr("192.0.2.6/24")
				if err := netlink.AddrAdd(link, v4); err != nil {
					return err
				}
				// Deliberately omit IFA_F_NODAD so the kernel starts DAD.
				v6, _ := netlink.ParseAddr("2001:db8:abcd:0:0a:bcff:fe00:0001/64")
				return netlink.AddrAdd(link, v6)
			})
			Expect(err).NotTo(HaveOccurred())

			prev := makePrev("192.0.2.6/24")

			// Immediately after AddrAdd the v6 is IFA_F_TENTATIVE — fallback
			// MUST NOT consider it.
			got, err := networking.GetIPFamilyByResultWithIface(prev, testNetns, "eth0")
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(netlink.FAMILY_V4),
				"tentative v6 should be filtered per RFC 4862 §5.4")

			// Poll up to 5s for DAD to clear; in practice ~1-2s.
			Eventually(func() int {
				got, err = networking.GetIPFamilyByResultWithIface(prev, testNetns, "eth0")
				if err != nil {
					return -2
				}
				return got
			}, 5*time.Second, 100*time.Millisecond).Should(Equal(netlink.FAMILY_ALL))
		})
	})
})
