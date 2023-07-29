// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	"github.com/containernetworking/plugins/pkg/ip"
	"github.com/containernetworking/plugins/pkg/ns"
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
	hostVethName, podVethName, currentInterface, interfacePrefix string
	HijackCIDR                                                   []string
	netns                                                        ns.NetNS
	hostVethHwAddress, podVethHwAddress                          net.HardwareAddr
	currentAddress                                               []netlink.Addr
	hostIPRouteForPod                                            []net.IP
}

// firstInvoke check if coordinator is first called and do some checks:
// underlay mode only works with underlay mode, which can't work with overlay
// mode, and which can't be called in first cni invoked by using multus's
// annotations: v1.multus-cni.io/default-network
func (c *coordinator) coordinatorFirstInvoke(podFirstInterface string) error {
	var err error
	switch c.tuneMode {
	case ModeUnderlay:
		c.firstInvoke = c.currentInterface == podFirstInterface
		// underlay mode can't work with calico/cilium(overlay)
		if !c.firstInvoke {
			var exist bool
			exist, err = networking.CheckInterfaceExist(c.netns, defaultUnderlayVethName)
			if err != nil {
				return fmt.Errorf("failed to CheckInterfaceExist: %v", err)
			}

			if !exist {
				return fmt.Errorf("in multi-NIC mode, underlay mode can only work with underlay mode. please check pod's multus annotations")
			}
		}
		return nil
	case ModeOverlay:
		// in overlay mode, it should no veth0 and currentInterface isn't eth0
		if c.currentInterface == podFirstInterface {
			return fmt.Errorf("in overlay mode, underlay mode can only work with underlay mode. please check pod's multus annotations")
		}

		exist, err := networking.CheckInterfaceExist(c.netns, defaultUnderlayVethName)
		if err != nil {
			return fmt.Errorf("failed to CheckInterfaceExist: %v", err)
		}

		if exist {
			return fmt.Errorf("in multi-NIC mode, overlay mode can't work with underlay mode. please check pod's multus annotations")
		}

		c.firstInvoke, err = networking.IsFirstModeOverlayInvoke(c.netns, c.interfacePrefix)
		return err
	case ModeDisable:
		return nil
	}

	return fmt.Errorf("unknown tuneMode: %s", c.tuneMode)
}

// getRuleNumber return the number of rule table corresponding to the previous interface from the given interface.
// the input format must be 'net+number'
// for example:
// input: net1, output: 100(eth0)
// input: net2, output: 101(net1)
func (c *coordinator) getRuleNumber(iface string) int {
	if iface == defaultOverlayVethName {
		return unix.RT_TABLE_MAIN
	}
	if !strings.HasPrefix(iface, c.interfacePrefix) {
		return -1
	}

	numStr := strings.Trim(iface, c.interfacePrefix)
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return -1
	}
	return defaultPodRuleTable + num - 1
}

// setupVeth sets up a pair of virtual ethernet devices. move one to the host and other
// one to container.
func (c *coordinator) setupVeth(containerID string) error {
	var containerInterface net.Interface
	err := c.netns.Do(func(hostNS ns.NetNS) error {
		var err error
		_, containerInterface, err = ip.SetupVethWithName(c.podVethName, getHostVethName(containerID), 1500, "", hostNS)
		if err != nil {
			return err
		}

		link, err := netlink.LinkByName(containerInterface.Name)
		if err != nil {
			return err
		}

		if err := netlink.LinkSetUp(link); err != nil {
			return fmt.Errorf("failed to set %q UP: %v", containerInterface.Name, err)
		}
		return nil
	})

	return err
}

// setupNeighborhood setup neighborhood tables for pod and host.
// equivalent to: `ip neigh add ....`
func (c *coordinator) setupNeighborhood(logger *zap.Logger) error {
	var err error
	c.hostVethHwAddress, c.podVethHwAddress, err = networking.GetHwAddressByName(c.netns, c.podVethName, c.hostVethName)
	if err != nil {
		logger.Error("failed to GetHwAddressByName", zap.Error(err))
		return fmt.Errorf("failed to GetHwAddressByName: %v", err)
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
		return fmt.Errorf("failed to setup neigh table, couldn't find host veth link: %v", err)
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
			return fmt.Errorf("failed to setup neigh table, couldn't find pod veth link: %v", err)
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

// setupRoutes setup hijack subnet routes for pod and host
// equivalent to: `ip route add $route table $ruleTable`
func (c *coordinator) setupHijackRoutes(logger *zap.Logger, ruleTable int) error {
	v4Gw, v6Gw, err := networking.GetGatewayIP(c.currentAddress)
	if err != nil {
		logger.Error("failed to GetGatewayIP", zap.Error(err))
		return err
	}

	logger.Debug("Debug setupHijackRoutes", zap.String("v4Gw", v4Gw.String()), zap.String("v6Gw", v6Gw.String()))

	err = c.netns.Do(func(_ ns.NetNS) error {
		// make sure that veth0/eth0 forwards traffic within the cluster
		// eq: ip route add <cluster/service cidr> dev veth0/eth0
		for _, hijack := range c.HijackCIDR {
			_, ipNet, err := net.ParseCIDR(hijack)
			if err != nil {
				logger.Error("Invalid Hijack Cidr", zap.String("Cidr", hijack), zap.Error(err))
				return err
			}

			if err := networking.AddRoute(logger, ruleTable, c.ipFamily, netlink.SCOPE_UNIVERSE, c.podVethName, ipNet, v4Gw, v6Gw); err != nil {
				logger.Error("failed to AddRoute for hijackCIDR", zap.String("Dst", ipNet.String()), zap.Error(err))
				return fmt.Errorf("failed to AddRoute for hijackCIDR: %v", err)
			}

			if c.tuneMode == ModeOverlay && c.firstInvoke {
				if err := networking.AddRoute(logger, unix.RT_TABLE_MAIN, c.ipFamily, netlink.SCOPE_UNIVERSE, c.podVethName, ipNet, v4Gw, v6Gw); err != nil {
					logger.Error("failed to AddRoute for hijackCIDR", zap.String("Dst", ipNet.String()), zap.Error(err))
					return fmt.Errorf("failed to AddRoute for hijackCIDR: %v", err)
				}
				logger.Debug("Add Route for hijackSubnet in pod successfully", zap.String("Dst", ipNet.String()))
			}

		}
		logger.Debug("AddRouteTable for localCIDRs successfully", zap.Strings("localCIDRs", c.HijackCIDR))

		return nil
	})
	return err
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
			if err = networking.AddRoute(logger, c.currentRuleTable, c.ipFamily, netlink.SCOPE_LINK, c.podVethName, ipNet, nil, nil); err != nil {
				logger.Error("failed to AddRoute for ipAddressOnNode", zap.Error(err))
				return fmt.Errorf("failed to AddRouteTable for ipAddressOnNode: %v", err)
			}

			if c.tuneMode == ModeOverlay && c.firstInvoke {
				if err = networking.AddRoute(logger, unix.RT_TABLE_MAIN, c.ipFamily, netlink.SCOPE_LINK, c.podVethName, ipNet, nil, nil); err != nil {
					logger.Error("failed to AddRoute for ipAddressOnNode", zap.Error(err))
					return fmt.Errorf("failed to AddRouteTable for ipAddressOnNode: %v", err)
				}
				logger.Debug("Add Route for hostAddress in pod successfully", zap.String("Dst", ipNet.String()))
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	for idx := range c.currentAddress {
		ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)

		// do any cleans dirty rule tables
		filterRule := netlink.NewRule()
		filterRule.Table = c.hostRuleTable
		filterRule.Dst = ipNet
		filtedRules, err := netlink.RuleListFiltered(netlink.FAMILY_V4, filterRule, netlink.RT_FILTER_DST)
		if err != nil {
			logger.Warn("failed to fetch rule list filter by RT_FILTER_DST, ignore clean dirty rule table")
		}

		for idx := range filtedRules {
			if err = netlink.RuleDel(&filtedRules[idx]); err != nil && !os.IsNotExist(err) {
				logger.Warn("failed to clean dirty rule table, it may cause the pod can't communicate with the node, please clean it up manually",
					zap.String("dirty rule table", filtedRules[idx].String()))
			} else {
				logger.Debug("successfully cleaned up the dirty rule table", zap.String("dirty rule table", filtedRules[idx].String()))
			}
		}

		if err = networking.AddToRuleTable(ipNet, c.hostRuleTable); err != nil {
			logger.Error("failed to AddToRuleTable", zap.String("Dst", ipNet.String()), zap.Error(err))
			return fmt.Errorf("failed to AddToRuleTable: %v", err)
		}

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
		if err = networking.AddRoute(logger, c.hostRuleTable, c.ipFamily, netlink.SCOPE_LINK, c.hostVethName, ipNet, nil, nil); err != nil {
			logger.Error("failed to AddRouteTable for preInterfaceIPAddress", zap.Error(err))
			return fmt.Errorf("failed to AddRouteTable for preInterfaceIPAddress: %v", err)
		}
		logger.Info("add route for to pod in host", zap.String("Dst", ipNet.String()))
	}

	return nil
}

// tunePodRoutes make sure that move all routes of podDefaultRouteNIC interface to main table, and move original routes
// in main table to new table
func (c *coordinator) tunePodRoutes(logger *zap.Logger, configDefaultRouteNIC string) error {
	if configDefaultRouteNIC == "" {
		// by default, We always think currentInterface as pod default router interface
		configDefaultRouteNIC = c.currentInterface
	}

	exist, err := networking.CheckInterfaceExist(c.netns, configDefaultRouteNIC)
	if err != nil {
		logger.Error("failed to CheckInterfaceExist", zap.String("interface", configDefaultRouteNIC), zap.Error(err))
		return fmt.Errorf("failed to CheckInterfaceExist: %v", err)
	}

	if !exist {
		return fmt.Errorf("podDefaultRouteNIC: %s don't exist in pod", configDefaultRouteNIC)
	}

	podDefaultRouteNIC, err := networking.GetDefaultRouteInterface(c.ipFamily, c.currentInterface, c.netns)
	if err != nil {
		logger.Error("failed to GetDefaultRouteInterface", zap.Error(err))
		return fmt.Errorf("failed to GetDefaultRouteInterface: %v", err)
	}

	if podDefaultRouteNIC == "" {
		logger.Warn("podDefaultRouteNIC no found in pod, ignore tuneRoutes")
		return nil
	}

	logger.Sugar().Infof("podDefaultRouteNIC: %v", podDefaultRouteNIC)

	// make sure that traffic sent from current interface to lookup table <ruleTable>
	// eq: ip rule add from <currentInterfaceIPAddress> lookup <ruleTable>
	err = c.netns.Do(func(_ ns.NetNS) error {
		defaultInterfaceAddress, err := networking.GetAddersByName(podDefaultRouteNIC, c.ipFamily)
		if err != nil {
			logger.Error("failed to GetAdders for podDefaultRouteNIC", zap.Error(err))
			return fmt.Errorf("failed to GetDefaultRouteInterface for podDefaultRouteNIC: %v", err)
		}

		logger.Sugar().Infof("defaultInterfaceAddress: %v", defaultInterfaceAddress)

		// get all routes of current interface
		currentInterfaceRoutes, err := networking.GetRoutesByName(c.currentInterface, c.ipFamily)
		if err != nil {
			logger.Error("failed to GetRoutesByName", zap.Error(err))
			return fmt.Errorf("failed to GetRoutesByName: %v", err)
		}

		logger.Sugar().Infof("currentInterfaceRoutes: %v", currentInterfaceRoutes)

		// get all routes of default route interface
		defaultInterfaceRoutes, err := networking.GetRoutesByName(podDefaultRouteNIC, c.ipFamily)
		if err != nil {
			logger.Error("failed to GetRoutesByName", zap.Error(err))
			return fmt.Errorf("failed to GetRoutesByName: %v", err)
		}

		logger.Sugar().Infof("defaultInterfaceRoutes: %v", defaultInterfaceRoutes)

		if configDefaultRouteNIC == c.currentInterface {
			for idx, route := range defaultInterfaceRoutes {
				if route.Dst != nil {
					if err := networking.AddToRuleTable(defaultInterfaceRoutes[idx].Dst, c.currentRuleTable); err != nil {
						logger.Error("failed to AddToRuleTable", zap.Error(err))
						return fmt.Errorf("failed to AddToRuleTable: %v", err)
					}
				}
			}

			for idx := range defaultInterfaceAddress {
				ipNet := networking.ConvertMaxMaskIPNet(defaultInterfaceAddress[idx].IP)
				err = networking.AddFromRuleTable(ipNet, c.currentRuleTable)
				if err != nil {
					logger.Error("failed to AddFromRuleTable", zap.Error(err))
					return err
				}
			}

			// move all routes of the specified interface to a new route table
			if err = networking.MoveRouteTable(logger, podDefaultRouteNIC, unix.RT_TABLE_MAIN, c.currentRuleTable, c.ipFamily); err != nil {
				return err
			}

		} else if configDefaultRouteNIC == podDefaultRouteNIC {
			for idx, route := range currentInterfaceRoutes {
				if route.Dst != nil {
					if err := networking.AddToRuleTable(currentInterfaceRoutes[idx].Dst, c.currentRuleTable); err != nil {
						logger.Error("failed to AddToRuleTable", zap.Error(err))
						return fmt.Errorf("failed to AddToRuleTable: %v", err)
					}
				}
			}

			for idx := range c.currentAddress {
				ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)
				err = networking.AddFromRuleTable(ipNet, c.currentRuleTable)
				if err != nil {
					logger.Error("failed to AddFromRuleTable", zap.Error(err))
					return err
				}
			}

			// move all routes of the specified interface from src rule table to dst route table
			if err = networking.MoveRouteTable(logger, c.currentInterface, unix.RT_TABLE_MAIN, c.currentRuleTable, c.ipFamily); err != nil {
				return err
			}
		} else {
			// that's mean there are more than 2 interfaces in pod, and
			// configDefaultRouteNIC's routes in a new rule table
			// we should move configDefaultRouteNIC's routes to main and
			// move currentInterface's routes to new rule table

			// move current interface's routes to new rule table
			for idx, route := range currentInterfaceRoutes {
				if route.Dst != nil {
					if err := networking.AddToRuleTable(currentInterfaceRoutes[idx].Dst, c.currentRuleTable); err != nil {
						logger.Error("failed to AddToRuleTable", zap.Error(err))
						return fmt.Errorf("failed to AddToRuleTable: %v", err)
					}
				}
			}

			for idx := range c.currentAddress {
				ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)
				err = networking.AddFromRuleTable(ipNet, c.currentRuleTable)
				if err != nil {
					logger.Error("failed to AddFromRuleTable", zap.Error(err))
					return err
				}
			}

			// move current interface's routes to new rule table
			if err = networking.MoveRouteTable(logger, c.currentInterface, unix.RT_TABLE_MAIN, c.currentRuleTable, c.ipFamily); err != nil {
				return err
			}

			routes, err := networking.GetRoutesByName(configDefaultRouteNIC, c.ipFamily)
			if err != nil {
				return fmt.Errorf("failed to GetRoutesByName for configDefaultRouteNIC: %v", err)
			}

			address, err := networking.GetAddersByName(configDefaultRouteNIC, c.ipFamily)
			if err != nil {
				return fmt.Errorf("failed to GetAddrs for configDefaultRouteNIC: %v", err)
			}

			ruleTable := c.getRuleNumber(configDefaultRouteNIC)
			if ruleTable < 0 {
				return fmt.Errorf("failed to getRuleNumber for podDefaultRouteNIC: podDefaultRouteNIC %s don't match NICPrefix %s", configDefaultRouteNIC, c.interfacePrefix)
			}

			// 1. cleanup ip rule to cidr for configDefaultRouteNIC interface
			for idx := range routes {
				if routes[idx].Dst != nil {
					if err = networking.DelToRuleTable(routes[idx].Dst, ruleTable); err != nil {
						return fmt.Errorf("failed to DelToRuleTable: %v", err)
					}
				}
			}

			// 2. cleanup ip rule from cidr for configDefaultRouteNIC interface
			for idx := range address {
				if routes[idx].Dst != nil {
					if err = networking.DelFromRuleTable(address[idx].IPNet, ruleTable); err != nil {
						return fmt.Errorf("failed to DelToRuleTable: %v", err)
					}
				}
			}

			// 3. move configDefaultRouteNIC interface's routes to main table
			if err = networking.MoveRouteTable(logger, configDefaultRouteNIC, ruleTable, unix.RT_TABLE_MAIN, c.ipFamily); err != nil {
				return err
			}
		}

		// for idx, _ := range c.hostIPRouteForPod {
		//	ipNet := networking.ConvertMaxMaskIPNet(c.hostIPRouteForPod[idx])
		//	if err = networking.DelToRuleTable(ipNet, c.hostRuleTable); err != nil {
		//		logger.Error("failed to AddToRuleTable", zap.String("Dst", ipNet.String()), zap.Error(err))
		//		// return fmt.Errorf("failed to AddToRuleTable: %v", err)
		//	}
		// }

		logger.Info("tunePodRoutes successfully", zap.String("configDefaultRouteInterface", configDefaultRouteNIC), zap.String("currentDefaultRouteInterface", podDefaultRouteNIC))
		return nil
	})

	if err != nil {
		logger.Error("failed to moveRouteTable for routeMoveInterface", zap.String("routeMoveInterface", configDefaultRouteNIC), zap.Error(err))
		return err
	}

	return nil
}

// makeReplyPacketViaVeth make sure that tcp replay packet is forward by veth0
// NOTE: underlay mode only.
func (c *coordinator) makeReplyPacketViaVeth(logger *zap.Logger) error {
	v4Gw, v6Gw, err := networking.GetGatewayIP(c.currentAddress)
	if err != nil {
		return fmt.Errorf("failed to get gateway ips: %v", err)
	}

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
			if err := networking.AddRuleTableWithMark(markInt, c.hostRuleTable, family); err != nil {
				return fmt.Errorf("failed to add rule table with mark: %v", err)
			}

			if err = networking.AddRoute(logger, c.hostRuleTable, family, netlink.SCOPE_UNIVERSE, c.podVethName, nil, v4Gw, v6Gw); err != nil {
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
			return fmt.Errorf("iptables ensureRule err: failed to set-xmark: %v", err)
		}

		_, err = ipt.EnsureRule(utiliptables.Append, utiliptables.TableMangle, utiliptables.ChainPrerouting, []string{
			"-m", "mark",
			"--mark", markStr,
			"-j", "CONNMARK",
			"--save-mark",
		}...)
		if err != nil {
			return fmt.Errorf("iptables ensureRule err: failed to save-mark: %v", err)
		}

		_, err = ipt.EnsureRule(utiliptables.Append, utiliptables.TableMangle, utiliptables.ChainOutput, []string{
			"-j", "CONNMARK",
			"--restore-mark",
		}...)
		if err != nil {
			return fmt.Errorf("iptables ensureRule err: failed to restore-mark: %v", err)
		}
	}
	return nil
}

func GetAllHostIPRouteForPod(c *coordinator, ipFamily int, allPodIp []netlink.Addr) (finalNodeIpList []net.IP, e error) {

	finalNodeIpList = []net.IP{}

OUTER1:
	// get node ip by `ip r get podIP`
	for _, item := range allPodIp {
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
		for _, k := range finalNodeIpList {
			if k.Equal(t) {
				continue OUTER1
			}
		}
		finalNodeIpList = append(finalNodeIpList, t)
	}

	var DefaultNodeInterfacesToExclude = []string{
		"docker.*", "cbr.*", "dummy.*",
		"virbr.*", "lxcbr.*", "veth.*", `^lo$`,
		`^cali.*`, "flannel.*", "kube-ipvs.*",
		"cni.*", "vx-submariner", "cilium*",
	}

	// get additional host ip
	additionalIp, err := networking.GetAllIPAddress(ipFamily, DefaultNodeInterfacesToExclude)
	if err != nil {
		return nil, fmt.Errorf("failed to get IPAddressOnNode: %v", err)
	}
OUTER2:
	for _, t := range additionalIp {
		if len(t.IP) == 0 {
			continue OUTER2
		}

		for _, k := range finalNodeIpList {
			if k.Equal(t.IP) {
				continue OUTER2
			}
		}
		if t.IP.To4() != nil {
			if ipFamily == netlink.FAMILY_V4 || ipFamily == netlink.FAMILY_ALL {
				finalNodeIpList = append(finalNodeIpList, t.IP)
			}
		} else {
			if ipFamily == netlink.FAMILY_V6 || ipFamily == netlink.FAMILY_ALL {
				finalNodeIpList = append(finalNodeIpList, t.IP)
			}
		}
	}

	return finalNodeIpList, nil
}
