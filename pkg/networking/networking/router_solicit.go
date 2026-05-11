// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking

import (
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/utils/sysctl"
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

// SolicitRouterAndWaitForSLAACv6 sends an ICMPv6 Router Solicitation on
// ifName from inside netns and polls AddrList until a non-link-local v6
// address appears, up to timeout. Used to drive SLAAC when the kernel has
// already dropped the periodic RAs that arrived before tuning set
// accept_ra=2 (with the pod inheriting forwarding=1, the kernel silently
// drops RAs at accept_ra<2 — see net/ipv6/addrconf.c::ipv6_accept_ra).
// Caller must ensure accept_ra is permissive on the iface before invoking
// (typically by chaining the upstream `tuning` plugin earlier in the NAD);
// otherwise the function exits without sending an RS. See #5618.
//
// Timeout splits between waiting for the iface's link-local to clear DAD
// (RFC 4862 §5.4: kernel refuses to source from a tentative address) and
// the RA/SLAAC round-trip. Empty return on timeout is not an error.
// Requires NET_RAW.
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

		// Gate: only solicit if the kernel would actually process the RA we
		// elicit. Skips v4-only setups (accept_ra=0 / accept_ra=1 with
		// forwarding=1) so they don't pay any latency. See
		// net/ipv6/addrconf.c::ipv6_accept_ra.
		if !kernelWouldAcceptRA(ifName) {
			return nil
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

// kernelWouldAcceptRA returns true when the Linux kernel's ipv6_accept_ra
// predicate (net/ipv6/addrconf.c) would accept a received RA on ifName.
// Mirrors the kernel logic exactly:
//
//	forwarding == 0 → accept iff accept_ra != 0
//	forwarding != 0 → accept iff accept_ra == 2
//
// Read failures are treated as "would not accept" (conservative: skip the
// solicit rather than spend ~3s sending an RS the kernel will ignore).
func kernelWouldAcceptRA(ifName string) bool {
	acceptRA, err := readIntSysctl("net/ipv6/conf/" + ifName + "/accept_ra")
	if err != nil {
		return false
	}
	forwarding, err := readIntSysctl("net/ipv6/conf/" + ifName + "/forwarding")
	if err != nil {
		return false
	}
	if forwarding == 0 {
		return acceptRA != 0
	}
	return acceptRA == 2
}

func readIntSysctl(path string) (int, error) {
	raw, err := sysctl.Sysctl(path)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(raw))
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
