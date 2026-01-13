// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	plugincmd "github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/spidernet-io/spiderpool/pkg/networking/sysctl"
	"github.com/spidernet-io/spiderpool/pkg/openapi"
)

func CmdAdd(args *skel.CmdArgs) (err error) {
	startTime := time.Now()

	k8sArgs := plugincmd.K8sArgs{}
	if err = types.LoadArgs(args.Args, &k8sArgs); nil != err {
		return fmt.Errorf("failed to load CNI ENV args: %w", err)
	}

	client, err := openapi.NewAgentOpenAPIUnixClient(constant.DefaultIPAMUnixSocketPath)
	if err != nil {
		return err
	}

	resp, err := client.Daemonset.GetCoordinatorConfig(daemonset.NewGetCoordinatorConfigParams().WithGetCoordinatorConfig(
		&models.GetCoordinatorArgs{
			PodName:      string(k8sArgs.K8S_POD_NAME),
			PodNamespace: string(k8sArgs.K8S_POD_NAMESPACE),
		},
	))
	if err != nil {
		return fmt.Errorf("failed to GetCoordinatorConfig: %w", err)
	}
	coordinatorConfig := resp.Payload

	conf, err := ParseConfig(args.StdinData, coordinatorConfig)
	if err != nil {
		return err
	}

	if conf.Mode == ModeDisable {
		return types.PrintResult(conf.PrevResult, conf.CNIVersion)
	}

	logger, err := logutils.SetupFileLogging(conf.LogOptions.LogLevel,
		conf.LogOptions.LogFilePath, conf.LogOptions.LogFileMaxSize,
		conf.LogOptions.LogFileMaxAge, conf.LogOptions.LogFileMaxCount)
	if err != nil {
		return fmt.Errorf("failed to init logger: %w ", err)
	}

	logger = logger.Named(BinNamePlugin).With(
		zap.String("Action", "ADD"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("Netns", args.Netns),
		zap.String("IfName", args.IfName),
		zap.String("PodName", string(k8sArgs.K8S_POD_NAME)),
		zap.String("PodNamespace", string(k8sArgs.K8S_POD_NAMESPACE)),
	)
	logger.Info(fmt.Sprintf("start to implement ADD command in %v mode", conf.Mode))
	logger.Debug(fmt.Sprintf("api configuration: %+v", *coordinatorConfig))
	logger.Debug("final configuration", zap.Any("conf", conf))

	// parse prevResult
	prevResult, err := current.GetResult(conf.PrevResult)
	if err != nil {
		logger.Error("failed to convert prevResult", zap.Error(err))
		return err
	}

	ipFamily, err := networking.GetIPFamilyByResult(prevResult)
	if err != nil {
		logger.Error("failed to GetIPFamilyByResult", zap.Error(err))
		return err
	}

	c := &coordinator{
		HijackCIDR:       conf.OverlayPodCIDR,
		hostRuleTable:    int(*conf.HostRuleTable),
		ipFamily:         ipFamily,
		currentInterface: args.IfName,
		tuneMode:         conf.Mode,
		vethLinkAddress:  conf.VethLinkAddress,
	}
	c.HijackCIDR = append(c.HijackCIDR, conf.ServiceCIDR...)
	c.HijackCIDR = append(c.HijackCIDR, conf.HijackCIDR...)

	c.netns, err = ns.GetNS(args.Netns)
	if err != nil {
		logger.Error(err.Error())
		return fmt.Errorf("failed to GetNS %q: %w", args.Netns, err)
	}
	defer func() { _ = c.netns.Close() }()

	c.hostNs, err = ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get current netns: %w", err)
	}
	defer func() { _ = c.hostNs.Close() }()
	logger.Sugar().Debugf("Get current host netns: %v", c.hostNs.Path())

	// checking if the nic is in up state
	logger.Sugar().Debugf("checking if %s is in up state", args.IfName)
	if err = c.checkNICState(args.IfName); err != nil {
		logger.Error("error to check pod's nic state", zap.Error(err))
		return fmt.Errorf("error to check pod's nic %s state: %w", args.Args, err)
	}

	// check if it's first time invoke
	err = c.coordinatorModeAndFirstInvoke(logger, conf.PodDefaultCniNic)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	// get all ip of pod
	var allPodIP []netlink.Addr
	err = c.netns.Do(func(netNS ns.NetNS) error {
		allPodIP, err = networking.GetAllIPAddress(ipFamily, []string{`^lo$`})
		if err != nil {
			logger.Error("failed to GetAllIPAddress in pod", zap.Error(err))
			return fmt.Errorf("failed to GetAllIPAddress in pod: %w", err)
		}
		return nil
	})
	if err != nil {
		logger.Error("failed to all ip of pod", zap.Error(err))
		return err
	}
	logger.Debug(fmt.Sprintf("all pod ip: %+v", allPodIP))

	// get ip addresses of the node
	c.hostIPRouteForPod, err = GetAllHostIPRouteForPod(c, ipFamily, allPodIP)
	if err != nil {
		logger.Error("failed to get IPAddressOnNode", zap.Error(err))
		return fmt.Errorf("failed to get IPAddressOnNode: %w", err)
	}
	logger.Debug(fmt.Sprintf("host IP for route to Pod: %+v", c.hostIPRouteForPod))

	// get basic info
	switch c.tuneMode {
	case ModeUnderlay:
		c.podVethName = defaultUnderlayVethName
		c.hostVethName = getHostVethName(args.ContainerID)
		if c.firstInvoke {
			err = c.setupVeth(logger, args.ContainerID)
			if err != nil {
				logger.Error("failed to create veth-pair device", zap.Error(err))
				return err
			}
			logger.Debug("Setup veth-pair device successfully", zap.String("hostVethPairName", getHostVethName(args.ContainerID)),
				zap.String("hostVethMac", c.hostVethHwAddress.String()), zap.String("podVethMac", c.podVethHwAddress.String()))
		}
	case ModeOverlay:
		c.podVethName = defaultOverlayVethName
		c.hostVethName, err = networking.GetHostVethName(c.netns, defaultOverlayVethName)
		if err != nil {
			logger.Error("failed to GetHostVethName", zap.Error(err))
			return err
		}
	case ModeDisable:
		logger.Info("TuneMode is disable, nothing to do")
		return types.PrintResult(conf.PrevResult, conf.CNIVersion)
	default:
		logger.Error("Unknown tuneMode", zap.String("invalid tuneMode", string(conf.Mode)))
		return fmt.Errorf("unknown tuneMode: %s", conf.Mode)
	}

	logger.Sugar().Infof("Get coordinator config: %+v", c)

	// get ips of this interface(preInterfaceName) from, including ipv4 and ipv6
	c.currentAddress, err = networking.IPAddressByName(c.netns, args.IfName, ipFamily)
	if err != nil {
		logger.Error(err.Error())
		return fmt.Errorf("failed to IPAddressByName for pod %s : %w", args.IfName, err)
	}

	logger.Debug("Get currentAddress", zap.Any("currentAddress", c.currentAddress))

	// Fixed Mac addresses must come after IP conflict detection, otherwise the switch learns to communicate
	// with the wrong Mac address when IP conflict detection fails
	if len(conf.MacPrefix) != 0 {
		hwAddr, err := networking.OverwriteHwAddress(logger, c.netns, conf.MacPrefix, args.IfName)
		if err != nil {
			return fmt.Errorf("failed to update hardware address for interface %s, maybe hardware_prefix(%s) is invalid: %w", args.IfName, conf.MacPrefix, err)
		}
		logger.Info("Fix mac address successfully", zap.String("interface", args.IfName), zap.String("macAddress", hwAddr))

		if err = c.netns.Do(func(_ ns.NetNS) error {
			return c.AnnounceIPs(logger)
		}); err != nil {
			logger.Error("failed to AnnounceIPs", zap.Error(err))
		}
	}

	// set txqueuelen
	if conf.TxQueueLen != nil && *conf.TxQueueLen > 0 {
		if err = networking.LinkSetTxqueueLen(args.IfName, int(*conf.TxQueueLen)); err != nil {
			return fmt.Errorf("failed to set %s txQueueLen to %v: %w", args.IfName, conf.TxQueueLen, err)
		}
	}

	// =================================

	if ipFamily != netlink.FAMILY_V4 {
		// ensure ipv6 is enable
		if err = c.netns.Do(func(nn ns.NetNS) error {
			return sysctl.EnableIpv6Sysctl(0)
		}); err != nil {
			logger.Error(err.Error())
			return err
		}
	}

	if *conf.PodRPFilter != -1 {
		if err = sysctl.SetSysctlRPFilter(c.netns, *conf.PodRPFilter); err != nil {
			logger.Error(err.Error())
			return err
		}
	}

	if err = c.setupNeighborhood(logger); err != nil {
		logger.Error("failed to setupNeighborhood", zap.Error(err))
		return err
	}

	allPodVethAddrs, err := networking.IPAddressByName(c.netns, defaultOverlayVethName, c.ipFamily)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	for idx := range allPodVethAddrs {
		copyAddr := allPodVethAddrs[idx]
		if c.v4PodOverlayNicAddr == nil && copyAddr.IP.To4() != nil {
			c.v4PodOverlayNicAddr = copyAddr.IPNet
			continue
		}

		if c.v6PodOverlayNicAddr == nil && copyAddr.IP.To16() != nil {
			// for ipv6, maybe kernel require the src must be from the device in
			// some kernel version.
			// refer to https://bugzilla.kernel.org/show_bug.cgi?id=107071
			c.v6PodOverlayNicAddr = nil
		}
	}

	logger.Debug("Show the addrs of pod's eth0", zap.Any("v4PodOverlayNicAddr", c.v4PodOverlayNicAddr), zap.Any("v6PodOverlayNicAddr", c.v6PodOverlayNicAddr))

	if err = c.setupHostRoutes(logger); err != nil {
		logger.Error(err.Error())
		return err
	}

	// get v4 and v6 gw for hijick route'gw
	for _, gw := range c.hostIPRouteForPod {
		copy := gw
		if copy.To4() != nil {
			if c.v4HijackRouteGw == nil && c.ipFamily != netlink.FAMILY_V6 {
				c.v4HijackRouteGw = copy
				logger.Debug("Get v4HijackRouteGw", zap.String("v4HijackRouteGw", c.v4HijackRouteGw.String()))
			}
		} else {
			if c.v6HijackRouteGw == nil && c.ipFamily != netlink.FAMILY_V4 {
				c.v6HijackRouteGw = copy
				logger.Debug("Get v6HijackRouteGw", zap.String("v6HijackRouteGw", c.v6HijackRouteGw.String()))
			}
		}
	}

	if err = c.setupHijackRoutes(logger, c.currentRuleTable); err != nil {
		logger.Error("failed to setupHijackRoutes", zap.Error(err))
		return err
	}

	if c.tuneMode == ModeUnderlay && c.firstInvoke {
		if err = c.makeReplyPacketViaVeth(logger); err != nil {
			logger.Error("failed to makeReplyPacketViaVeth", zap.Error(err))
			return fmt.Errorf("failed to makeReplyPacketViaVeth: %w", err)
		} else {
			logger.Sugar().Infof("Successfully to ensure reply packet is forward by veth0")
		}
	}

	if conf.TunePodRoutes != nil && *conf.TunePodRoutes && (!c.firstInvoke || c.tuneMode == ModeOverlay) {
		logger.Debug("Try to tune pod routes")
		if err = c.tunePodRoutes(logger, conf.PodDefaultRouteNIC); err != nil {
			logger.Error("failed to tunePodRoutes", zap.Error(err))
			return fmt.Errorf("failed to tunePodRoutes: %w", err)
		}
		logger.Debug("Success to tune pod routes")
	}

	logger.Sugar().Infof("coordinator end, time cost: %v", time.Since(startTime))
	return types.PrintResult(conf.PrevResult, conf.CNIVersion)
}
