// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networking

import (
	"fmt"
	"net"
	"os"

	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

var defaultRulePriority = 1000

// GetRoutesByName return all routes is belonged to specify interface
// filter by family also
func GetRoutesByName(iface string, ipfamily int) (routes []netlink.Route, err error) {
	var link netlink.Link
	if iface != "" {
		link, err = netlink.LinkByName(iface)
		if err != nil {
			return nil, err
		}
	}

	return netlink.RouteList(link, ipfamily)
}

func GetDefaultGatewayByName(iface string, ipfamily int) ([]net.IP, error) {
	routes, err := GetRoutesByName("", ipfamily)
	if err != nil {
		return nil, err
	}

	link, err := netlink.LinkByName(iface)
	if err != nil {
		return nil, err
	}

	gws := make([]net.IP, 0)
	for _, route := range routes {
		if route.LinkIndex == link.Attrs().Index {
			if route.Dst == nil || route.Dst.IP.Equal(net.IPv4zero) || route.Dst.IP.Equal(net.IPv6zero) {
				gws = append(gws, route.Gw)
			}
		} else {
			if len(route.MultiPath) > 0 {
				for _, r := range route.MultiPath {
					if r.LinkIndex == link.Attrs().Index {
						gws = append(gws, r.Gw)
						break
					}
				}
			}
		}
	}
	return gws, nil
}

func AddToRuleTable(dst *net.IPNet, ruleTable int) error {
	rule := netlink.NewRule()
	rule.Table = ruleTable
	rule.Dst = dst
	return netlink.RuleAdd(rule)
}

func DelToRuleTable(dst *net.IPNet, ruleTable int) error {
	rule := netlink.NewRule()
	rule.Table = ruleTable
	rule.Dst = dst
	return netlink.RuleDel(rule)
}

func AddRuleTableWithMark(mark, ruleTable, ipFamily int) error {
	rule := netlink.NewRule()
	rule.Mark = mark
	rule.Table = ruleTable
	rule.Family = ipFamily
	rule.Priority = defaultRulePriority
	return netlink.RuleAdd(rule)
}

// AddFromRuleTable add route rule for calico/cilium cidr(ipv4 and ipv6)
// Equivalent to: `ip rule add from <cidr> `
func AddFromRuleTable(src *net.IPNet, ruleTable int) error {
	rule := netlink.NewRule()
	rule.Table = ruleTable
	rule.Src = src
	return netlink.RuleAdd(rule)
}

// DelFromRuleTable equivalent to: `ip rule del from <cidr> lookup <ruletable>`
func DelFromRuleTable(src *net.IPNet, ruleTable int) error {
	rule := netlink.NewRule()
	rule.Table = ruleTable
	rule.Src = src
	return netlink.RuleDel(rule)
}

// AddRoute add static route to specify rule table
func AddRoute(logger *zap.Logger, ruleTable, ipFamily int, scope netlink.Scope, iface string, src, dst *net.IPNet, v4Gw, v6Gw net.IP) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	route := &netlink.Route{
		LinkIndex: link.Attrs().Index,
		Scope:     scope,
		Dst:       dst,
		Table:     ruleTable,
	}

	if src != nil {
		route.Src = src.IP
	}

	switch ipFamily {
	case netlink.FAMILY_V4:
		if v4Gw != nil {
			route.Gw = v4Gw
		}
	case netlink.FAMILY_V6:
		if v6Gw != nil {
			route.Gw = v6Gw
		}
	case netlink.FAMILY_ALL:
		if dst != nil && dst.IP.To4() != nil && v4Gw != nil {
			route.Gw = v4Gw
		}

		if dst != nil && dst.IP.To4() == nil && v6Gw != nil {
			route.Gw = v6Gw
		}
	default:
		return fmt.Errorf("unknown ipFamily %v", ipFamily)
	}

	if err = netlink.RouteAdd(route); err != nil && !os.IsExist(err) {
		logger.Error("failed to RouteAdd", zap.String("route", route.String()), zap.Error(err))
		return fmt.Errorf("failed to add route table(%v): %w", route.String(), err)
	}
	return nil
}

func GetLinkIndexAndRoutes(iface string, ipfamily int) (int, []netlink.Route, error) {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return -1, nil, err
	}

	routes, err := netlink.RouteList(nil, ipfamily)
	if err != nil {
		return -1, nil, err
	}

	return link.Attrs().Index, routes, nil
}

// CopyDefaultRoute found the default route of pod's eth0 nic, and copy this
// to dstRuleTable.
func CopyDefaultRoute(logger *zap.Logger, iface string, srcRuleTable, podOverlayDefaultRouteRuleTable, ipfamily int) error {
	logger.Debug("Debug MoveRouteTable", zap.String("interface", iface),
		zap.Int("srcRuleTable", srcRuleTable), zap.Int("dstRuleTable", podOverlayDefaultRouteRuleTable))

	linkIndex, routes, err := GetLinkIndexAndRoutes(iface, ipfamily)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	for _, route := range routes {
		// only handle route tables from table main
		if route.Table != srcRuleTable {
			continue
		}

		// ignore local link route
		if route.Dst.String() == "fe80::/64" {
			continue
		}

		if err = moveRouteTable(linkIndex, srcRuleTable, podOverlayDefaultRouteRuleTable, true, route, logger); err != nil {
			return err
		}

	}
	return nil
}

// MoveRouteTable move all routes of the specified interface to a new route table
// Equivalent: `ip route del <route>` and `ip r route add <route> <table>`
func MoveRouteTable(logger *zap.Logger, iface string, srcRuleTable, dstRuleTable, ipfamily int) error {
	logger.Debug("Debug MoveRouteTable", zap.String("interface", iface),
		zap.Int("srcRuleTable", srcRuleTable), zap.Int("dstRuleTable", dstRuleTable))
	linkIndex, routes, err := GetLinkIndexAndRoutes(iface, ipfamily)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	for _, route := range routes {
		// only handle route tables from table main
		if route.Table != srcRuleTable {
			continue
		}

		// ignore local link route
		if route.Dst.String() == "fe80::/64" {
			continue
		}

		if err = moveRouteTable(linkIndex, srcRuleTable, dstRuleTable, false, route, logger); err != nil {
			return err
		}

	}
	return nil
}

// moveRouteTable move route table from srcRuleTable to dstRuleTable. NOTE: if copyOverlayDefaultRoute is true,
// only add the default route to host rule table and exit in advance.
func moveRouteTable(linkIndex, srcRuleTable, dstRuleTable int, onlyCopyOverlayDefaultRoute bool, route netlink.Route, logger *zap.Logger) error {
	var err error
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
			if err = netlink.RouteAdd(&defaultRoute); err != nil && !os.IsExist(err) {
				logger.Error("failed to copy overlay default route to hostRuleTable", zap.String("route", defaultRoute.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteAdd (%+v) to new table: %+w", defaultRoute, err)
			}

			if onlyCopyOverlayDefaultRoute {
				// only copy overlay default route, don't need delete the default route
				logger.Debug("Only add the default route, Do not delete it")
				return nil
			}

			// Del the default route from main
			defaultRoute.Table = srcRuleTable
			logger.Debug("try to delete the route", zap.String("Route", defaultRoute.String()))
			if err = netlink.RouteDel(&defaultRoute); err != nil {
				logger.Error("failed to RouteDel in main", zap.String("route", defaultRoute.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteDel %s in main table: %+w", defaultRoute.String(), err)
			}
			return nil
		}

		if onlyCopyOverlayDefaultRoute {
			// only copy overlay default route, don't need add non-default routes
			return nil
		}

		// we need copy the all routes in main table of the podDefaultRouteNic to dstRuleTable.
		// Otherwise, we don't know how to forward the packet send from the nic
		staticRoute := netlink.Route{
			Dst:       route.Dst,
			Src:       route.Src,
			Gw:        route.Gw,
			LinkIndex: linkIndex,
			Scope:     route.Scope,
			Table:     dstRuleTable,
		}
		if err = netlink.RouteAdd(&staticRoute); err != nil && !os.IsExist(err) {
			logger.Error("failed to add the route table", zap.String("route", staticRoute.String()), zap.Error(err))
			return fmt.Errorf("failed to add the route table (%+v): %+w", route, err)
		}
		logger.Debug("MoveRoute to new table successfully", zap.String("Route", staticRoute.String()))
		return nil
	}

	// in high kernel, if pod has multi ipv6 default routes, all default routes
	// will be put in MultiPath
	/*
		{
			Gw: [{Ifindex: 3 Weight: 1 Gw: fd00:10:7::103 Flags: []} {Ifindex: 5 Weight: 1 Gw: fd00:10:6::100 Flags: []}]}"
		}
	*/
	if len(route.MultiPath) == 0 {
		return nil
	}

	var generatedRoute, deletedRoute *netlink.Route
	// get generated default Route for new table
	for _, v := range route.MultiPath {
		logger.Debug("Found IPv6 Default Route", zap.String("Route", route.String()),
			zap.Int("v.LinkIndex", linkIndex), zap.Int("link.Attrs().Index", linkIndex))
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

	if err = netlink.RouteAdd(generatedRoute); err != nil && !os.IsExist(err) {
		logger.Error("failed to RouteAdd for IPv6 to new table", zap.String("route", route.String()), zap.Error(err))
		return fmt.Errorf("failed to RouteAdd for IPv6 (%+v) to new table: %+w", route.String(), err)
	}

	if onlyCopyOverlayDefaultRoute {
		return nil
	}

	logger.Debug("Deleting IPv6 DefaultRoute", zap.String("deletedRoute", deletedRoute.String()))
	if err := netlink.RouteDel(deletedRoute); err != nil {
		logger.Error("failed to RouteDel for IPv6", zap.String("Route", route.String()), zap.Error(err))
		return fmt.Errorf("failed to RouteDel %v for IPv6: %+w", route.String(), err)
	}

	return nil
}

// GetDefaultRouteInterface returns the name of the NIC where the default route is located
// if filterInterface not be empty, return first default route interface
// otherwise filter filterInterface
func GetDefaultRouteInterface(ipfamily int, filterInterface string, netns ns.NetNS) (string, error) {
	var defaultInterface string
	err := netns.Do(func(_ ns.NetNS) error {
		links, err := netlink.LinkList()
		if err != nil {
			return err
		}

		routes, err := netlink.RouteList(nil, ipfamily)
		if err != nil {
			return err
		}

		for _, l := range links {
			if l.Attrs().Name == "lo" || l.Attrs().Name == filterInterface {
				continue
			}

			for _, route := range routes {
				if route.LinkIndex == l.Attrs().Index {
					if route.Dst == nil || route.Dst.IP.Equal(net.IPv4zero) || route.Dst.IP.Equal(net.IPv6zero) {
						defaultInterface = l.Attrs().Name
						return nil
					}
				} else {
					for _, mp := range route.MultiPath {
						if mp.LinkIndex == l.Attrs().Index {
							defaultInterface = l.Attrs().Name
							return nil
						}
					}
				}
			}
		}
		return nil
	})
	return defaultInterface, err
}

func ConvertMaxMaskIPNet(nip net.IP) *net.IPNet {
	mIPNet := &net.IPNet{
		IP: nip,
	}
	if nip.To4() != nil {
		mIPNet.Mask = net.CIDRMask(32, 32)
	} else {
		mIPNet.Mask = net.CIDRMask(128, 128)
	}
	return mIPNet
}
