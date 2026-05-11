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
})
