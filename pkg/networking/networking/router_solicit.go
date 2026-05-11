// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking

import (
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/mdlayher/ndp"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// allIPv6RoutersMulticast is `ff02::2`, the multicast group every IPv6 router
// on a link joins. Routers respond to a Router Solicitation sent here with a
// Router Advertisement (RFC 4861 §6.3.5).
var allIPv6RoutersMulticast = netip.MustParseAddr("ff02::2")

// solicitPollInterval bounds how often we re-scan the iface while waiting for
// link-local DAD to clear and then for the SLAAC v6 to appear. The kernel
// emits the events asynchronously so polling is the simplest reader.
const solicitPollInterval = 100 * time.Millisecond

// SolicitRouterAndWaitForSLAACv6 sends a Router Solicitation on ifName from
// inside netns and polls AddrList until a non-link-local, non-tentative IPv6
// address appears, up to timeout. Returns the discovered addresses (already
// filtered by getAdders), or an empty slice on timeout. A timed-out poll is
// NOT an error — it just means the network didn't deliver a usable SLAAC v6
// in the available budget.
//
// Solves the CNI ADD race where the SLAAC v6 has not landed on the pod iface
// by the time coordinator runs (issue #5618):
//
//   - The Multus chain runs `macvlan` → `tuning` → `coordinator`. Macvlan
//     brings the iface up while `accept_ra` is at its inherited default. The
//     pod inherits `net.ipv4.ip_forward=1` (k3s default, also common on
//     other CNIs); under that combination the kernel's `ipv6_accept_ra`
//     predicate returns false when `accept_ra<2` (`net/ipv6/addrconf.c`),
//     so no link-up RS is sent and any periodic RA is ignored.
//   - The tuning plugin then sets `accept_ra=2`, but the kernel does NOT
//     auto-retransmit RS on the change; SLAAC waits for the next periodic
//     RA, which can be seconds to minutes away.
//   - Coordinator runs ~ms after tuning, before that next periodic RA.
//
// An explicit RS to `ff02::2` forces on-link router(s) to respond with an RA
// immediately (RFC 4861 §6.2.6, §6.3.5). With `accept_ra=2` now active the
// kernel processes the RA and runs SLAAC; the resulting GUA appears within
// ~ms (plus DAD time on the GUA, which is filtered upstream by getAdders).
//
// timeout caps the whole operation. Internally split between two waits:
//
//  1. Up to half the budget for the iface's link-local address to clear DAD.
//     The Linux kernel refuses to source a packet from a tentative address
//     (RFC 4862 §5.4: "A tentative address is not considered 'assigned to
//     an interface' in the traditional sense"), so an RS sent before LL DAD
//     completes returns EADDRNOTAVAIL.
//  2. The remainder for the RA/SLAAC round-trip plus GUA DAD.
//
// Required capabilities: NET_RAW (raw ICMPv6 socket). Coordinator already
// runs with it as a CNI plugin.
func SolicitRouterAndWaitForSLAACv6(netns ns.NetNS, ifName string, timeout time.Duration) ([]netlink.Addr, error) {
	if netns == nil || ifName == "" {
		return nil, fmt.Errorf("netns and ifName are required")
	}

	var found []netlink.Addr
	err := netns.Do(func(_ ns.NetNS) error {
		link, err := netlink.LinkByName(ifName)
		if err != nil {
			return fmt.Errorf("LinkByName %q: %w", ifName, err)
		}

		// Phase 1: wait for a non-tentative link-local. RFC 4861 §4.1
		// recommends the RS source be a link-local of the sending iface; the
		// kernel refuses sendmsg from `::` when no usable source is on the
		// iface (and tentative addresses don't count per RFC 4862 §5.4).
		llDeadline := time.Now().Add(timeout / 2)
		if !waitForUsableLinkLocal(link, llDeadline) {
			// No usable link-local in budget. Caller treats empty as "no v6
			// yet" and falls back to v4. Not an error condition.
			return nil
		}

		// Phase 2: send the RS and poll for a non-link-local v6 to appear.
		ifi, err := net.InterfaceByName(ifName)
		if err != nil {
			return fmt.Errorf("InterfaceByName %q: %w", ifName, err)
		}
		conn, _, err := ndp.Listen(ifi, ndp.LinkLocal)
		if err != nil {
			return fmt.Errorf("open ICMPv6 socket on %q: %w", ifName, err)
		}
		defer func() { _ = conn.Close() }()

		rs := &ndp.RouterSolicitation{
			// Include the source link-layer address option (RFC 4861 §4.1:
			// SHOULD include on link layers with addresses when source is
			// not unspecified) so routers can preload their neighbor cache.
			Options: []ndp.Option{
				&ndp.LinkLayerAddress{
					Direction: ndp.Source,
					Addr:      ifi.HardwareAddr,
				},
			},
		}
		if err := conn.WriteTo(rs, nil, allIPv6RoutersMulticast); err != nil {
			return fmt.Errorf("send RS on %q: %w", ifName, err)
		}

		deadline := llDeadline.Add(timeout - timeout/2)
		for {
			addrs, err := getAdders(link, netlink.FAMILY_V6)
			if err != nil {
				return fmt.Errorf("scan v6 on %q: %w", ifName, err)
			}
			if len(addrs) > 0 {
				found = addrs
				return nil
			}
			if time.Now().After(deadline) {
				return nil
			}
			time.Sleep(solicitPollInterval)
		}
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// waitForUsableLinkLocal polls link's v6 addresses until at least one
// link-local is no longer IFA_F_TENTATIVE / IFA_F_DADFAILED, or the deadline
// passes. Returns true if a usable LL was observed.
func waitForUsableLinkLocal(link netlink.Link, deadline time.Time) bool {
	for {
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V6)
		if err == nil {
			for _, a := range addrs {
				if !a.IP.IsLinkLocalUnicast() {
					continue
				}
				if a.Flags&(unix.IFA_F_TENTATIVE|unix.IFA_F_DADFAILED) == 0 {
					return true
				}
			}
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(solicitPollInterval)
	}
}
