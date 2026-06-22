// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"net"
	"os"

	"github.com/cilium/cilium/pkg/mac"
	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/mdlayher/ndp"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	utiliptables "k8s.io/kubernetes/pkg/util/iptables"
	"k8s.io/utils/exec"

	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
)

type coordinator struct {
	firstInvoke                                                  bool
	ipFamily, currentRuleTable, hostRuleTable                    int
	tuneMode                                                     Mode
	hostVethName, podVethName, vethLinkAddress, currentInterface string
	v4HijackRouteGw, v6HijackRouteGw                             net.IP
	HijackCIDR                                                   []string
	netns, hostNs                                                ns.NetNS
	hostVethHwAddress, podVethHwAddress                          net.HardwareAddr
	currentAddress                                               []netlink.Addr
	v4PodOverlayNicAddr, v6PodOverlayNicAddr                     *net.IPNet
	hostIPRouteForPod                                            []net.IP
}

func (c *coordinator) autoModeToSpecificMode(mode Mode, podFirstInterface string, vethExist bool) error {
	if mode != ModeAuto {
		return nil
	}

	if c.currentInterface == podFirstInterface {
		c.tuneMode = ModeUnderlay
		return nil
	}

	if vethExist {
		c.tuneMode = ModeUnderlay
	} else {
		c.tuneMode = ModeOverlay
	}

	return nil
}

// firstInvoke check if coordinator is first called and do some checks:
// underlay mode only works with underlay mode, which can't work with overlay
// mode, and which can't be called in first cni invoked by using multus's
// annotations: v1.multus-cni.io/default-network
func (c *coordinator) coordinatorModeAndFirstInvoke(logger *zap.Logger, podFirstInterface string) error {
	vethExist, err := networking.CheckInterfaceExist(c.netns, defaultUnderlayVethName)
	if err != nil {
		return fmt.Errorf("failed to CheckInterfaceExist: %w", err)
	}

	links, err := networking.GetUPLinkList(c.netns)
	if err != nil {
		return fmt.Errorf("failed to get link list: %w", err)
	}

	for _, l := range links {
		logger.Info("===debug link", zap.String("link", l.Attrs().Name))
	}

	if c.tuneMode == ModeAuto {
		if err = c.autoModeToSpecificMode(ModeAuto, podFirstInterface, vethExist); err != nil {
			return err
		}
		logger.Sugar().Infof("Successfully auto detect mode, change mode from auto to %v", c.tuneMode)
	}

	switch c.tuneMode {
	case ModeUnderlay:
		c.firstInvoke = c.currentInterface == podFirstInterface
		// underlay mode can't work with calico/cilium(overlay)
		if !c.firstInvoke && !vethExist {
			return fmt.Errorf("when creating interface %s in underlay mode, it detects that the auxiliary interface %s was not created by previous interface. please enable coordinator plugin in previous interface", c.currentInterface, podFirstInterface)
		}

		// ensure that each NIC has a separate policy routing table number
		if c.firstInvoke {
			c.currentRuleTable = defaultPodRuleTable
		} else {
			// for non-eth0 or non first-underlay nic, Policy routing
			// table numbers are cumulative based on the number of NICs
			// for example:
			// there are veth0, eth0,net1,net2 nic, the policy routing table numbers
			// of net2 is:  4 + 97 == 101.
			c.currentRuleTable = len(links) + 97
		}
		return nil
	case ModeOverlay:
		// in overlay mode, it should no veth0 and currentInterface isn't eth0
		if c.currentInterface == podFirstInterface {
			logger.Sugar().Warnf("when creating interface %s in overlay mode, it detects that the current interface is first interface named %s, skip coordinator route tuning", c.currentInterface, podFirstInterface)
			c.tuneMode = ModeDisable
			return nil
		}

		if vethExist {
			return fmt.Errorf("when creating interface %s in overlay mode, it detects that the auxiliary interface %s of underlay mode exists. It seems that the previous interface work in underlay mode. ", c.currentInterface, defaultUnderlayVethName)
		}

		// if pod has only eth0 and net1, the first invoke is true
		c.firstInvoke = len(links) == 2
		if c.firstInvoke {
			c.currentRuleTable = defaultPodRuleTable
		} else {
			// for non-eth0 or non first-underlay nic, Policy routing
			// table numbers are cumulative based on the number of NICs
			// for example:
			// there are eth0,net1,net2 nic, the policy routing table numbers
			// of net2 is:  3 + 98 == 101.
			c.currentRuleTable = 98 + len(links)
		}
		return nil
	case ModeDisable:
		return nil
	}

	return fmt.Errorf("unknown tuneMode: %s", c.tuneMode)
}

func (c *coordinator) checkNICState(iface string) error {
	return c.netns.Do(func(netNS ns.NetNS) error {
		link, err := netlink.LinkByName(iface)
		if err != nil {
			return err
		}

		if link.Attrs().Flags != net.FlagUp {
			return netlink.LinkSetUp(link)
		}
		return nil
	})
}

// setupVeth sets up a pair of virtual ethernet devices. move one to the host and other
// one to container.
func (c *coordinator) setupVeth(logger *zap.Logger, containerID string) error {
	// systemd 242+ tries to set a "persistent" MAC addr for any virtual device
	// by default (controlled by MACAddressPolicy). As setting happens
	// asynchronously after a device has been created, ep.Mac and ep.HostMac
	// can become stale which has a serious consequence - the kernel will drop
	// any packet sent to/from the endpoint. However, we can trick systemd by
	// explicitly setting MAC addrs for both veth ends. This sets
	// addr_assign_type for NET_ADDR_SET which prevents systemd from changing
	// the addrs.
	podVethMAC, err := mac.GenerateRandMAC()
	if err != nil {
		return fmt.Errorf("unable to generate podVeth mac addr: %w", err)
	}

	hostVethMac, err := mac.GenerateRandMAC()
	if err != nil {
		return fmt.Errorf("unable to generate hostVeth mac addr: %w", err)
	}

	var containerInterface net.Interface
	hostVethName := getHostVethName(containerID)
	err = c.netns.Do(func(hostNS ns.NetNS) error {
		_, containerInterface, err = ip.SetupVethWithName(c.podVethName, hostVethName, 1500, podVethMAC.String(), hostNS)
		if err != nil {
			return err
		}

		link, err := netlink.LinkByName(containerInterface.Name)
		if err != nil {
			return err
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to set %q UP: %w", containerInterface.Name, err)
		}

		if c.ipFamily == netlink.FAMILY_V6 {
			// set an address to veth to fix work with istio
			// set only when not ipv6 only
			return nil
		}

		if c.vethLinkAddress == "" {
			return nil
		}

		if err = netlink.AddrAdd(link, &netlink.Addr{
			IPNet: &net.IPNet{
				IP:   net.ParseIP(c.vethLinkAddress),
				Mask: net.CIDRMask(32, 32),
			},
		}); err != nil {
			return fmt.Errorf("failed to add ip address to veth0: %w", err)
		}
		return nil
	})

	hostVethLink, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return err
	}

	if err = netlink.LinkSetHardwareAddr(hostVethLink, net.HardwareAddr(hostVethMac)); err != nil {
		return fmt.Errorf("failed to set host veth mac: %w", err)
	}

	logger.Debug("Successfully to set veth mac", zap.String("podVethMac", podVethMAC.String()), zap.String("hostVethMac", hostVethMac.String()))
	return err
}

// setupNeighborhood setup neighborhood tables for pod and host.
// equivalent to: `ip neigh add ....`
func (c *coordinator) setupNeighborhood(logger *zap.Logger) error {
	var err error
	c.hostVethHwAddress, c.podVethHwAddress, err = networking.GetHwAddressByName(c.netns, c.podVethName, c.hostVethName)
	if err != nil {
		logger.Error("failed to GetHwAddressByName", zap.Error(err))
		return fmt.Errorf("failed to GetHwAddressByName: %w", err)
	}

	logger.Debug("Debug setupNeighborhood", zap.String("HostVethName", c.hostVethName),
		zap.String("HostVethMac", c.hostVethHwAddress.String()),
		zap.String("PodVethName", c.podVethName),
		zap.String("PodVethMac", c.podVethHwAddress.String()))

	// do any cleans?
	nList, err := netlink.NeighList(0, c.ipFamily)
	if err != nil {
		logger.Warn("failed to get NeighList, ignore clean dirty neigh table")
	}

	hostVethlink, err := netlink.LinkByName(c.hostVethName)
	if err != nil {
		logger.Error("failed to setup neigh table, couldn't find host veth link", zap.Error(err))
		return fmt.Errorf("failed to setup neigh table, couldn't find host veth link: %w", err)
	}

	for idx := range nList {
		for _, ipAddr := range c.currentAddress {
			if nList[idx].IP.Equal(ipAddr.IP) {
				if err = netlink.NeighDel(&nList[idx]); err != nil && !os.IsNotExist(err) {
					logger.Warn("failed to clean dirty neigh table, it may cause the pod can't communicate with the node, please clean it up manually",
						zap.String("dirty neigh table", nList[idx].String()))
				} else {
					logger.Debug("successfully cleaned up the dirty neigh table", zap.String("dirty neigh table", nList[idx].String()))
				}
				break
			}
		}
	}

	for _, ipAddr := range c.currentAddress {
		if err = networking.AddStaticNeighborTable(hostVethlink.Attrs().Index, ipAddr.IP, c.podVethHwAddress); err != nil {
			logger.Error(err.Error())
			return err
		}
	}

	if !c.firstInvoke {
		return nil
	}

	err = c.netns.Do(func(_ ns.NetNS) error {
		podVethlink, err := netlink.LinkByName(c.podVethName)
		if err != nil {
			logger.Error("failed to setup neigh table, couldn't find pod veth link", zap.Error(err))
			return fmt.Errorf("failed to setup neigh table, couldn't find pod veth link: %w", err)
		}

		for _, ipAddr := range c.hostIPRouteForPod {
			if err := networking.AddStaticNeighborTable(podVethlink.Attrs().Index, ipAddr, c.hostVethHwAddress); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	logger.Debug("Setup Neighborhood Table Successfully")
	return err
}

// setupHijackRoutes adds routes for overlay pod CIDRs, service CIDRs, and
// user-configured hijackCIDR entries. These routes are not added to the
// current underlay policy table (non-main table), including auto mode that
// resolved to overlay because the pod default NIC is provided by an overlay
// CNI.
func (c *coordinator) setupHijackRoutes(logger *zap.Logger, ruleTable int) error {
	err := c.netns.Do(func(_ ns.NetNS) error {
		// make sure that veth0/eth0 forwards traffic to cluster and hijack CIDRs.
		// eq: ip route add <cluster/service/hijack cidr> dev veth0/eth0
		for _, hijack := range c.HijackCIDR {
			nip, ipNet, err := net.ParseCIDR(hijack)
			if err != nil {
				logger.Error("Invalid Hijack Cidr", zap.String("Cidr", hijack), zap.Error(err))
				return err
			}

			var src *net.IPNet
			if nip.To4() != nil {
				if c.v4HijackRouteGw == nil {
					logger.Warn("ignore adding hijack routing table(ipv4), due to ipv4 gateway is nil", zap.String("IPv4 Hijack cidr", hijack))
					continue
				}
				src = c.v4PodOverlayNicAddr
			}

			if nip.To4() == nil {
				if c.v6HijackRouteGw == nil {
					logger.Warn("ignore adding hijack routing table(ipv6), due to ipv6 gateway is nil", zap.String("IPv6 Hijack cidr", hijack))
					continue
				}
				src = c.v6PodOverlayNicAddr
			}

			if c.firstInvoke {
				ruleTable = unix.RT_TABLE_MAIN
			}

			if ruleTable != unix.RT_TABLE_MAIN {
				logger.Debug("Skip adding hijack routes for underlay policy table", zap.Int("Table", ruleTable), zap.Bool("FirstInvoke", c.firstInvoke), zap.String("Mode", string(c.tuneMode)))
				continue
			}

			if err := networking.AddRoute(logger, ruleTable, c.ipFamily, netlink.SCOPE_UNIVERSE, c.podVethName, src, ipNet, c.v4HijackRouteGw, c.v6HijackRouteGw); err != nil {
				logger.Error("failed to AddRoute for hijackCIDR", zap.String("Dst", ipNet.String()), zap.Error(err))
				return fmt.Errorf("failed to AddRoute for hijackCIDR: %w", err)
			}
			logger.Debug("AddRouteTable for localCIDRs successfully", zap.String("hijick cidr", hijack), zap.Int("Table", ruleTable))
		}
		return nil
	})
	return err
}

func (c *coordinator) underlayPolicyRouteSkipCIDRs(logger *zap.Logger) ([]*net.IPNet, error) {
	skipCIDRs := make([]*net.IPNet, 0, len(c.HijackCIDR))
	for _, hijack := range c.HijackCIDR {
		_, dst, err := net.ParseCIDR(hijack)
		if err != nil {
			logger.Error("Invalid Hijack Cidr", zap.String("Cidr", hijack), zap.Error(err))
			return nil, err
		}
		skipCIDRs = append(skipCIDRs, dst)
	}

	return skipCIDRs, nil
}

func shouldSkipRoute(route netlink.Route, skipCIDRs []*net.IPNet) bool {
	if route.Dst == nil {
		return false
	}

	for _, skipCIDR := range skipCIDRs {
		if networking.IPNetEqual(route.Dst, skipCIDR) {
			return true
		}
	}

	return false
}

func shouldKeepRouteInMain(route netlink.Route) bool {
	return route.Dst != nil && route.Gw == nil
}

func (c *coordinator) shouldAddHostRouteToPodPolicyTable(ruleTable int) bool {
	return ruleTable == unix.RT_TABLE_MAIN
}

func moveRouteTableWithSkip(linkIndex, srcRuleTable, dstRuleTable int, route netlink.Route, deleteNonDefaultRoute bool, skipCIDRs []*net.IPNet, logger *zap.Logger) error {
	if route.LinkIndex == linkIndex {
		if route.Dst == nil || route.Dst.IP.Equal(net.IPv4zero) || route.Dst.IP.Equal(net.IPv6zero) {
			defaultRoute := netlink.Route{
				Dst:       route.Dst,
				Table:     dstRuleTable,
				LinkIndex: linkIndex,
				Scope:     route.Scope,
				Gw:        route.Gw,
			}
			logger.Debug("try to add the route", zap.String("Route", defaultRoute.String()))
			if err := netlink.RouteAdd(&defaultRoute); err != nil && !os.IsExist(err) {
				logger.Error("failed to copy default route to policy table", zap.String("route", defaultRoute.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteAdd (%+v) to new table: %+w", defaultRoute, err)
			}

			defaultRoute.Table = srcRuleTable
			logger.Debug("try to delete the route", zap.String("Route", defaultRoute.String()))
			if err := netlink.RouteDel(&defaultRoute); err != nil {
				logger.Error("failed to RouteDel in main", zap.String("route", defaultRoute.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteDel %s in main table: %+w", defaultRoute.String(), err)
			}
			return nil
		}

		if shouldSkipRoute(route, skipCIDRs) {
			logger.Debug("skip copying route to underlay policy table", zap.String("Route", route.String()), zap.Int("Table", dstRuleTable))
			return nil
		}

		staticRoute := netlink.Route{
			Dst:       route.Dst,
			Src:       route.Src,
			Gw:        route.Gw,
			LinkIndex: linkIndex,
			Scope:     route.Scope,
			Table:     dstRuleTable,
		}
		if err := netlink.RouteAdd(&staticRoute); err != nil && !os.IsExist(err) {
			logger.Error("failed to add the route table", zap.String("route", staticRoute.String()), zap.Error(err))
			return fmt.Errorf("failed to add the route table (%+v): %+w", route, err)
		}
		logger.Debug("MoveRoute to new table successfully", zap.String("Route", staticRoute.String()))

		if deleteNonDefaultRoute && !shouldKeepRouteInMain(route) {
			staticRoute.Table = srcRuleTable
			logger.Debug("try to delete the route", zap.String("Route", staticRoute.String()))
			if err := netlink.RouteDel(&staticRoute); err != nil && !os.IsNotExist(err) {
				logger.Error("failed to RouteDel in main", zap.String("route", staticRoute.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteDel %s in main table: %+w", staticRoute.String(), err)
			}
		} else if deleteNonDefaultRoute {
			logger.Debug("keep direct subnet route in main table", zap.String("Route", route.String()))
		}
		return nil
	}

	if len(route.MultiPath) == 0 {
		return nil
	}

	var generatedRoute, deletedRoute *netlink.Route
	for _, v := range route.MultiPath {
		if v.LinkIndex == linkIndex {
			generatedRoute = &netlink.Route{
				LinkIndex: v.LinkIndex,
				Gw:        v.Gw,
				Table:     dstRuleTable,
				MTU:       route.MTU,
			}
			deletedRoute = &netlink.Route{
				LinkIndex: v.LinkIndex,
				Gw:        v.Gw,
				Table:     srcRuleTable,
			}
			break
		}
	}

	if generatedRoute == nil {
		return nil
	}

	if err := netlink.RouteAdd(generatedRoute); err != nil && !os.IsExist(err) {
		logger.Error("failed to RouteAdd for IPv6 to new table", zap.String("route", route.String()), zap.Error(err))
		return fmt.Errorf("failed to RouteAdd for IPv6 (%+v) to new table: %+w", route.String(), err)
	}

	logger.Debug("Deleting IPv6 DefaultRoute", zap.String("deletedRoute", deletedRoute.String()))
	if err := netlink.RouteDel(deletedRoute); err != nil {
		logger.Error("failed to RouteDel for IPv6", zap.String("Route", route.String()), zap.Error(err))
		return fmt.Errorf("failed to RouteDel %v for IPv6: %+w", route.String(), err)
	}

	return nil
}

func (c *coordinator) movePodRoutesToPolicyTable(logger *zap.Logger, iface string, deleteNonDefaultRoute bool) error {
	skipCIDRs, err := c.underlayPolicyRouteSkipCIDRs(logger)
	if err != nil {
		return err
	}
	if len(skipCIDRs) == 0 && !deleteNonDefaultRoute {
		return networking.MoveRouteTable(logger, iface, unix.RT_TABLE_MAIN, c.currentRuleTable, c.ipFamily)
	}

	logger.Debug("Debug MoveRouteTable with skip CIDRs", zap.String("interface", iface),
		zap.Int("srcRuleTable", unix.RT_TABLE_MAIN), zap.Int("dstRuleTable", c.currentRuleTable), zap.Any("skipCIDRs", skipCIDRs))

	linkIndex, routes, err := networking.GetLinkIndexAndRoutes(iface, c.ipFamily)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	for _, route := range routes {
		if route.Table != unix.RT_TABLE_MAIN {
			continue
		}

		if route.Dst != nil && route.Dst.String() == "fe80::/64" {
			continue
		}

		if err = moveRouteTableWithSkip(linkIndex, unix.RT_TABLE_MAIN, c.currentRuleTable, route, deleteNonDefaultRoute, skipCIDRs, logger); err != nil {
			return err
		}
	}

	return nil
}

// setupHostRoutes create routes for all host IPs, make sure that traffic to
// pod's host is forward to veth pair device.
func (c *coordinator) setupHostRoutes(logger *zap.Logger) error {
	var err error
	err = c.netns.Do(func(_ ns.NetNS) error {
		// traffic sent to the pod its node is forwarded via veth0/eth0
		// eq: "ip r add <ipAddressOnNode> dev veth0/eth0 table <ruleTable>"
		for _, hostAddress := range c.hostIPRouteForPod {
			ipNet := networking.ConvertMaxMaskIPNet(hostAddress)
			var src *net.IPNet
			if hostAddress.To4() != nil {
				src = c.v4PodOverlayNicAddr
			} else {
				src = c.v6PodOverlayNicAddr
			}

			if c.shouldAddHostRouteToPodPolicyTable(c.currentRuleTable) {
				if err = networking.AddRoute(logger, c.currentRuleTable, c.ipFamily, netlink.SCOPE_LINK, c.podVethName, src, ipNet, nil, nil); err != nil {
					logger.Error("failed to AddRoute for ipAddressOnNode", zap.Error(err))
					return err
				}
			} else {
				logger.Debug("Skip adding host routes for underlay policy table",
					zap.String("Dst", ipNet.String()),
					zap.Int("Table", c.currentRuleTable),
					zap.Bool("FirstInvoke", c.firstInvoke),
					zap.String("Mode", string(c.tuneMode)))
			}

			if c.firstInvoke {
				if err = networking.AddRoute(logger, unix.RT_TABLE_MAIN, c.ipFamily, netlink.SCOPE_LINK, c.podVethName, src, ipNet, nil, nil); err != nil {
					logger.Error("failed to AddRoute for ipAddressOnNode", zap.Error(err))
					return err
				}
				logger.Debug("Add Route for hostAddress in pod successfully", zap.String("Dst", ipNet.String()))
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	var ipFamilies []int
	if c.ipFamily == netlink.FAMILY_ALL {
		ipFamilies = append(ipFamilies, netlink.FAMILY_V4, netlink.FAMILY_V6)
	} else {
		ipFamilies = append(ipFamilies, c.ipFamily)
	}

	// make sure `ip rule from all lookup 500 pref 32765` exist
	rule := netlink.NewRule()
	rule.Table = c.hostRuleTable
	rule.Priority = defaultHostRulePriority
	for _, ipfamily := range ipFamilies {
		rule.Family = ipfamily
		if err = netlink.RuleAdd(rule); err != nil && !os.IsExist(err) {
			logger.Error("failed to Add ToRuleTable for host", zap.String("rule", rule.String()), zap.Error(err))
			return fmt.Errorf("failed to Add ToRuleTable for host(%+v): %w", rule.String(), err)
		}
	}

	for idx := range c.currentAddress {
		ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)

		// do any cleans dirty route tables
		filterRoute := &netlink.Route{
			Dst:   ipNet,
			Table: c.hostRuleTable,
		}

		filterRoutes, err := netlink.RouteListFiltered(c.ipFamily, filterRoute, netlink.RT_FILTER_TABLE)
		if err != nil {
			logger.Warn("failed to fetch route list filter by RT_FILTER_DST, ignore clean dirty route table")
		}

		for idx := range filterRoutes {
			if networking.IPNetEqual(filterRoutes[idx].Dst, ipNet) {
				if err = netlink.RouteDel(&filterRoutes[idx]); err != nil && !os.IsNotExist(err) {
					logger.Warn("failed to clean dirty route table, it may cause the pod can't communicate with the node, please clean it up manually",
						zap.String("dirty route table", filterRoutes[idx].String()))
				} else {
					logger.Debug("successfully cleaned up the dirty route table", zap.String("dirty route table", filterRoutes[idx].String()))
				}
			}
		}

		// set routes for host
		// equivalent: ip add  <chainedIPs> dev <hostVethName> table  on host
		if err = networking.AddRoute(logger, c.hostRuleTable, c.ipFamily, netlink.SCOPE_LINK, c.hostVethName, nil, ipNet, nil, nil); err != nil {
			logger.Error("failed to AddRouteTable for preInterfaceIPAddress", zap.Error(err))
			return fmt.Errorf("failed to AddRouteTable for preInterfaceIPAddress: %w", err)
		}
		logger.Info("add route for to pod in host", zap.String("Dst", ipNet.String()))
	}

	return nil
}

// tunePodRoutes make sure that move all routes of podDefaultRouteNIC interface to main table, and move original routes
// in main table to new table
func (c *coordinator) tunePodRoutes(logger *zap.Logger, configDefaultRouteNIC string) error {
	var err error
	var podDefaultRouteNIC, moveRouteInterface string
	podDefaultRouteNIC, err = networking.GetDefaultRouteInterface(c.ipFamily, c.currentInterface, c.netns)
	if err != nil {
		logger.Error("failed to GetDefaultRouteInterface", zap.Error(err))
		return fmt.Errorf("failed to GetDefaultRouteInterface: %w", err)
	}

	if podDefaultRouteNIC == "" {
		// the current interface's default route no found, we can keep all routes of
		// this nic in main table, and don't tune the routes
		logger.Warn("podDefaultRouteNIC no found in pod, ignore tuneRoutes")
		return nil
	}

	if configDefaultRouteNIC == "" || configDefaultRouteNIC == podDefaultRouteNIC {
		// configDefaultRouteNIC is empty by default, and we always keep the all routes of the
		// first NIC is in main and move the all routes of non-first NIC to policy routing table.
		// see https://github.com/spidernet-io/spiderpool/issues/2176.
		configDefaultRouteNIC = podDefaultRouteNIC
		moveRouteInterface = c.currentInterface
	} else {
		exist, err := networking.CheckInterfaceExist(c.netns, configDefaultRouteNIC)
		if err != nil {
			logger.Error("failed to CheckInterfaceExist", zap.String("interface", configDefaultRouteNIC), zap.Error(err))
			return fmt.Errorf("failed to CheckInterfaceExist: %w", err)
		}

		if !exist {
			return fmt.Errorf("podDefaultRouteNIC: %s don't exist in pod", configDefaultRouteNIC)
		}
		moveRouteInterface = podDefaultRouteNIC
	}

	logger.Debug("Start Move Pod's routes", zap.String("configDefaultRouteNIC", configDefaultRouteNIC), zap.String("moveRouteInterface", moveRouteInterface))

	// make sure that traffic sent from current interface to lookup table <ruleTable>
	// eq: ip rule add from <currentInterfaceIPAddress> lookup <ruleTable>
	err = c.netns.Do(func(_ ns.NetNS) error {
		defaultInterfaceAddress, err := networking.GetAddersByName(podDefaultRouteNIC, c.ipFamily)
		if err != nil {
			logger.Error("failed to GetAdders for podDefaultRouteNIC", zap.Error(err))
			return fmt.Errorf("failed to GetDefaultRouteInterface for podDefaultRouteNIC: %w", err)
		}

		logger.Sugar().Infof("defaultInterfaceAddress: %v", defaultInterfaceAddress)

		if configDefaultRouteNIC == c.currentInterface {
			for idx := range defaultInterfaceAddress {
				ipNet := networking.ConvertMaxMaskIPNet(defaultInterfaceAddress[idx].IP)
				err = networking.AddFromRuleTable(ipNet, c.currentRuleTable)
				if err != nil {
					logger.Error("failed to AddFromRuleTable", zap.Error(err))
					return err
				}
			}
		} else {
			for idx := range c.currentAddress {
				ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)
				err = networking.AddFromRuleTable(ipNet, c.currentRuleTable)
				if err != nil {
					logger.Error("failed to AddFromRuleTable", zap.Error(err))
					return err
				}
			}

			if c.tuneMode == ModeOverlay && c.firstInvoke && configDefaultRouteNIC != podDefaultRouteNIC {
				// Copy the calico or cilium default route to the next policy table
				// after the current underlay NIC table to fix the problem of
				// inconsistent routes, the pod forwards the response packet from net1 (macvlan)
				// when it sends the response packet. but the request packet comes from eth0(calico).
				// see https://github.com/spidernet-io/spiderpool/issues/3683

				// copy to the table after current underlay NIC's table
				podOverlayDefaultRouteRuleTable := c.currentRuleTable + 1
				for idx := range defaultInterfaceAddress {
					ipNet := networking.ConvertMaxMaskIPNet(defaultInterfaceAddress[idx].IP)
					err = networking.AddFromRuleTable(ipNet, podOverlayDefaultRouteRuleTable)
					if err != nil {
						logger.Error("failed to AddFromRuleTable", zap.Error(err))
						return err
					}
				}

				// move all routes of the specified interface to a new route table
				if err = networking.CopyDefaultRoute(logger, defaultOverlayVethName, unix.RT_TABLE_MAIN, podOverlayDefaultRouteRuleTable, c.ipFamily); err != nil {
					return err
				}
			}

		}
		// move all routes of the specified interface to a new route table
		if moveRouteInterface == c.currentInterface {
			err = c.movePodRoutesToPolicyTable(logger, moveRouteInterface, true)
		} else {
			err = c.movePodRoutesToPolicyTable(logger, moveRouteInterface, false)
		}
		if err != nil {
			return err
		}

		logger.Info("tunePodRoutes successfully")
		return nil
	})
	if err != nil {
		logger.Error("failed to moveRouteTable for routeMoveInterface", zap.Error(err))
		return err
	}

	return nil
}

// makeReplyPacketViaVeth make sure that tcp replay packet is forward by veth0
// NOTE: underlay mode only.
func (c *coordinator) makeReplyPacketViaVeth(logger *zap.Logger) error {
	var iptablesInterface []utiliptables.Interface
	var ipFamily []int
	execer := exec.New()
	markInt := getMarkInt(defaultMarkBit)
	switch c.ipFamily {
	case netlink.FAMILY_V4:
		iptablesInterface = append(iptablesInterface, utiliptables.New(execer, utiliptables.ProtocolIPv4))
		ipFamily = append(ipFamily, netlink.FAMILY_V4)
	case netlink.FAMILY_V6:
		iptablesInterface = append(iptablesInterface, utiliptables.New(execer, utiliptables.ProtocolIPv6))
		ipFamily = append(ipFamily, netlink.FAMILY_V6)
	case netlink.FAMILY_ALL:
		iptablesInterface = append(iptablesInterface, utiliptables.New(execer, utiliptables.ProtocolIPv4))
		iptablesInterface = append(iptablesInterface, utiliptables.New(execer, utiliptables.ProtocolIPv6))
		ipFamily = append(ipFamily, netlink.FAMILY_V4)
		ipFamily = append(ipFamily, netlink.FAMILY_V6)
	}

	return c.netns.Do(func(_ ns.NetNS) error {
		if err := c.ensureIPtablesRule(iptablesInterface); err != nil {
			return err
		}

		for _, family := range ipFamily {
			if err := networking.AddRuleTableWithMark(markInt, defaultPodRuleTable, family); err != nil && !os.IsExist(err) {
				return fmt.Errorf("failed to add rule table with mark: %w", err)
			}

			var src *net.IPNet
			if family == netlink.FAMILY_V4 {
				src = c.v4PodOverlayNicAddr
			} else {
				src = c.v6PodOverlayNicAddr
			}

			if err := networking.AddRoute(logger, defaultPodRuleTable, family, netlink.SCOPE_UNIVERSE, c.podVethName, src, nil, c.v4HijackRouteGw, c.v6HijackRouteGw); err != nil {
				return err
			}
		}
		return nil
	})
}

// getHostVethName select the first 11 characters of the containerID for the host veth.
func getHostVethName(containerID string) string {
	return fmt.Sprintf("veth%s", containerID[:min(len(containerID))])
}

func min(len int) int {
	if len > 11 {
		return 11
	}
	return len
}

func getMarkInt(markBit int) int {
	return 1 << markBit
}

func getMarkString(mark int) string {
	return fmt.Sprintf("%#08x", mark)
}

func (c *coordinator) ensureIPtablesRule(iptablesInterfaces []utiliptables.Interface) error {
	markStr := getMarkString(getMarkInt(0))
	for _, ipt := range iptablesInterfaces {
		if ipt == nil {
			continue
		}
		_, err := ipt.EnsureRule(utiliptables.Append, utiliptables.TableMangle, utiliptables.ChainPrerouting, []string{
			"-i", defaultUnderlayVethName,
			"-m", "conntrack",
			"--ctstate", "NEW",
			"-j", "MARK",
			"--set-xmark", markStr,
		}...)
		if err != nil {
			return fmt.Errorf("iptables ensureRule err: failed to set-xmark: %w", err)
		}

		_, err = ipt.EnsureRule(utiliptables.Append, utiliptables.TableMangle, utiliptables.ChainPrerouting, []string{
			"-m", "mark",
			"--mark", markStr,
			"-j", "CONNMARK",
			"--save-mark",
		}...)
		if err != nil {
			return fmt.Errorf("iptables ensureRule err: failed to save-mark: %w", err)
		}

		_, err = ipt.EnsureRule(utiliptables.Append, utiliptables.TableMangle, utiliptables.ChainOutput, []string{
			"-j", "CONNMARK",
			"--restore-mark",
		}...)
		if err != nil {
			return fmt.Errorf("iptables ensureRule err: failed to restore-mark: %w", err)
		}
	}
	return nil
}

func GetAllHostIPRouteForPod(c *coordinator, ipFamily int, allPodIP []netlink.Addr) (finalNodeIPList []net.IP, e error) {
	finalNodeIPList = []net.IP{}

OUTER1:
	// get node ip by `ip r get podIP`
	for _, item := range allPodIP {
		var t net.IP
		v4Gw, v6Gw, err := networking.GetGatewayIP([]netlink.Addr{item})
		if err != nil {
			return nil, fmt.Errorf("failed to GetGatewayIP for pod ip %+v : %+v ", item, zap.Error(err))
		}
		if len(v4Gw) > 0 && (ipFamily == netlink.FAMILY_V4 || ipFamily == netlink.FAMILY_ALL) {
			t = v4Gw
		} else if len(v6Gw) > 0 && (ipFamily == netlink.FAMILY_V6 || ipFamily == netlink.FAMILY_ALL) {
			t = v6Gw
		}
		for _, k := range finalNodeIPList {
			if k.Equal(t) {
				continue OUTER1
			}
		}
		finalNodeIPList = append(finalNodeIPList, t)
	}

	DefaultNodeInterfacesToExclude := []string{
		"^docker.*", "^cbr.*", "^dummy.*",
		"^virbr.*", "^lxcbr.*", "^veth.*", `^lo$`,
		`^cali.*`, "^flannel.*", "^kube-ipvs.*",
		"^cni.*", "^vx-submariner", "^cilium*",
	}

	// get additional host ip
	additionalIP, err := networking.GetAllIPAddress(ipFamily, DefaultNodeInterfacesToExclude)
	if err != nil {
		return nil, fmt.Errorf("failed to get IPAddressOnNode: %w", err)
	}
OUTER2:
	for _, t := range additionalIP {
		if len(t.IP) == 0 {
			continue OUTER2
		}

		for _, k := range finalNodeIPList {
			if k.Equal(t.IP) {
				continue OUTER2
			}
		}
		if t.IP.To4() != nil {
			if ipFamily == netlink.FAMILY_V4 || ipFamily == netlink.FAMILY_ALL {
				finalNodeIPList = append(finalNodeIPList, t.IP)
			}
		} else {
			if ipFamily == netlink.FAMILY_V6 || ipFamily == netlink.FAMILY_ALL {
				finalNodeIPList = append(finalNodeIPList, t.IP)
			}
		}
	}

	return finalNodeIPList, nil
}

func (c *coordinator) AnnounceIPs(logger *zap.Logger) error {
	l, err := netlink.LinkByName(c.currentInterface)
	if err != nil {
		return err
	}

	for _, addr := range c.currentAddress {
		if addr.IP.To4() != nil {
			// send an gratuitous arp to announce the new mac address
			if err = networking.SendARPReuqest(l, addr.IP, addr.IP); err != nil {
				logger.Error("failed to send gratuitous arps", zap.Error(err))
			} else {
				logger.Debug("Send gratuitous arps successfully", zap.String("interface", c.currentInterface))
			}
		} else {
			ifi, err := net.InterfaceByName(c.currentInterface)
			if err != nil {
				return fmt.Errorf("failed to InterfaceByName %s: %w", c.currentInterface, err)
			}

			ndpClient, _, err := ndp.Listen(ifi, ndp.LinkLocal)
			if err != nil {
				return fmt.Errorf("failed to init ndp client: %w", err)
			}
			defer func() { _ = ndpClient.Close() }()
			if err = networking.SendUnsolicitedNeighborAdvertisement(addr.IP, ifi, ndpClient); err != nil {
				logger.Error("failed to send unsolicited neighbor advertisements", zap.Error(err))
			} else {
				logger.Debug("Send unsolicited neighbor advertisements successfully", zap.String("interface", c.currentInterface))
			}
		}
	}
	return nil
}
