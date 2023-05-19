package networking

import (
	"fmt"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"net"
	"os"
)

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

	routes, err = netlink.RouteList(link, ipfamily)
	if err != nil {
		return nil, err
	}
	return
}

func AddToRuleTable(dst *net.IPNet, ruleTable int) error {
	rule := netlink.NewRule()
	rule.Table = ruleTable
	rule.Dst = dst
	if err := netlink.RuleAdd(rule); err != nil {
		return err
	}
	return nil
}

func DelToRuleTable(dst *net.IPNet, ruleTable int) error {
	rule := netlink.NewRule()
	rule.Table = ruleTable
	rule.Dst = dst
	if err := netlink.RuleDel(rule); err != nil {
		return err
	}
	return nil
}

// AddFromRuleTable add route rule for calico/cilium cidr(ipv4 and ipv6)
// Equivalent to: `ip rule add from <cidr> `
func AddFromRuleTable(src *net.IPNet, ruleTable int) error {
	rule := netlink.NewRule()
	rule.Table = ruleTable
	rule.Src = src
	if err := netlink.RuleAdd(rule); err != nil {
		return err
	}
	return nil
}

// DelFromRuleTable equivalent to: `ip rule del from <cidr> lookup <ruletable>`
func DelFromRuleTable(src *net.IPNet, ruleTable int) error {
	rule := netlink.NewRule()
	rule.Table = ruleTable
	rule.Src = src
	if err := netlink.RuleAdd(rule); err != nil {
		return err
	}
	return nil
}

// AddRoute add static route to specify rule table
func AddRoute(logger *zap.Logger, ruleTable int, scope netlink.Scope, iface string, dst *net.IPNet, v4Gw, v6Gw net.IP) error {
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

	if dst.IP.To4() != nil && v4Gw != nil {
		route.Gw = v4Gw
	}

	if dst.IP.To4() == nil && v6Gw != nil {
		route.Gw = v6Gw
	}

	if err = netlink.RouteAdd(route); err != nil && !os.IsExist(err) {
		logger.Error("failed to RouteAdd", zap.String("route", route.String()), zap.Error(err))
		return err
	}
	return nil
}

// MoveRouteTable move all routes of the specified interface to a new route table
// Equivalent: `ip route del <route>` and `ip r route add <route> <table>`
func MoveRouteTable(logger *zap.Logger, iface string, srcRuleTable, dstRuleTable, ipfamily int) error {
	link, err := netlink.LinkByName(iface)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	routes, err := netlink.RouteList(nil, ipfamily)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	for _, route := range routes {

		// only handle route tables from table main
		if route.Table != srcRuleTable {
			continue
		}

		// ingore local link route
		if route.Dst.String() == "fe80::/64" {
			continue
		}

		if route.LinkIndex == link.Attrs().Index {
			if err = netlink.RouteDel(&route); err != nil {
				logger.Error("failed to RouteDel in main", zap.String("route", route.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteDel %s in main table: %+v", route.String(), err)
			}
			logger.Debug("Del the route from main successfully", zap.String("Route", route.String()))

			route.Table = dstRuleTable
			if err = netlink.RouteAdd(&route); err != nil && os.IsExist(err) {
				logger.Error("failed to RouteAdd in new table ", zap.String("route", route.String()), zap.Error(err))
				return fmt.Errorf("failed to RouteAdd (%+v) to new table: %+v", route, err)
			}
			logger.Debug("MoveRoute to new table successfully", zap.String("Route", route.String()))
		} else {
			// especially for ipv6 default route
			if len(route.MultiPath) == 0 {
				continue
			}

			// get generated default Route for new table
			for _, v := range route.MultiPath {
				if v.LinkIndex == link.Attrs().Index {
					logger.Debug("Found IPv6 Default Route", zap.String("Route", route.String()))
					if err := netlink.RouteDel(&route); err != nil {
						logger.Error("failed to RouteDel for IPv6", zap.String("Route", route.String()), zap.Error(err))
						return fmt.Errorf("failed to RouteDel %v for IPv6: %+v", route.String(), err)
					}

					route.Table = dstRuleTable
					if err = netlink.RouteAdd(&route); err != nil && !os.IsExist(err) {
						logger.Error("failed to RouteAdd for IPv6 to new table", zap.String("route", route.String()), zap.Error(err))
						return fmt.Errorf("failed to RouteAdd for IPv6 (%+v) to new table: %+v", route.String(), err)
					}
					break
				}
			}
		}
	}
	return nil
}

// GetDefaultRouteInterface returns the name of the NIC where the default route is located
// if filterInterface not be empty, return first default route interface
// otherwise filter filterInterface
func GetDefaultRouteInterface(filterInterface string, ipfamily int) (string, error) {
	routes, err := GetRoutesByName("", ipfamily)
	if err != nil {
		return "", err
	}

	for idx, _ := range routes {
		if routes[idx].Dst == nil {
			// found default route
			link, err := netlink.LinkByIndex(routes[idx].LinkIndex)
			if err != nil {
				return "", err
			}

			if filterInterface != "" && link.Attrs().Name == filterInterface {
				continue
			}
			return link.Attrs().Name, nil
		}
	}
	return "", fmt.Errorf("DefaultRouteInterface no found")
}

func IsRuleMiss(netns ns.NetNS, rule int) (bool, error) {
	var rules []netlink.Rule
	var err error

	err = netns.Do(func(netNS ns.NetNS) error {
		rules, err = netlink.RuleList(netlink.FAMILY_ALL)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return false, err
	}

	for idx, _ := range rules {
		if rules[idx].Table == rule {
			return false, nil
		}
	}

	return true, nil
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
