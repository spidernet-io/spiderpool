// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package integration_test

// Real-netns coverage for GetIPFamilyByIface; rationale for the subpackage
// split is in doc.go.

import (
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
				addr.Flags |= unix.IFA_F_NODAD
				if err := netlink.AddrAdd(link, addr); err != nil {
					return err
				}
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	}

	DescribeTable(
		"GetIPFamilyByIface against a real interface",
		func(ifaceV4, ifaceV6 []string, wantFamily int, wantErr bool) {
			setupDummyIface(ifaceV4, ifaceV6)

			gotFamily, err := networking.GetIPFamilyByIface(testNetns, "eth0")
			if wantErr {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(gotFamily).To(Equal(wantFamily))
		},
		Entry("v4 + kernel SLAAC v6 → FAMILY_ALL (the bug scenario)",
			[]string{"192.0.2.6/24"}, []string{"2001:db8:abcd:0:0a:bcff:fe00:0001/64"},
			netlink.FAMILY_ALL, false),
		Entry("v4 only (kernel auto-assigned LL filtered) → FAMILY_V4",
			[]string{"192.0.2.6/24"}, nil,
			netlink.FAMILY_V4, false),
		Entry("v6 only → FAMILY_V6",
			nil, []string{"2001:db8:abcd:0:0a:bcff:fe00:0001/64"},
			netlink.FAMILY_V6, false),
		Entry("no IPs on iface → error",
			nil, nil,
			-1, true),
	)

	It("errors when the iface doesn't exist in the netns", func() {
		family, err := networking.GetIPFamilyByIface(testNetns, "does-not-exist")
		Expect(err).To(HaveOccurred())
		Expect(family).To(Equal(-1))
	})
})
