// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"golang.org/x/sys/unix"
	"time"

	"github.com/spidernet-io/spiderpool/internal/version"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/ipchecking"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/spidernet-io/spiderpool/pkg/networking/sysctl"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func CmdAdd(args *skel.CmdArgs) (err error) {
	startTime := time.Now()
	conf, err := ParseConfig(args.StdinData)
	if err != nil {
		return err
	}

	logger, err := logutils.SetupFileLogging(conf.LogOptions.LogLevel,
		conf.LogOptions.LogFilePath, conf.LogOptions.LogFileMaxSize,
		conf.LogOptions.LogFileMaxAge, conf.LogOptions.LogFileMaxCount)
	if err != nil {
		return fmt.Errorf("failed to init logger: %v ", err)
	}

	logger.Info("coordinator starting", zap.String("Version", version.CoordinatorBuildDateVersion()), zap.String("Branch", version.CoordinatorGitBranch()),
		zap.String("Commit", version.CoordinatorGitCommit()),
		zap.String("Build time", version.CoordinatorBuildDate()),
		zap.String("Go Version", version.GoString()))

	logger.Sugar().Infof("coordinator run in mode: %v", conf.TuneMode)

	logger = logger.Named(BinNamePlugin).With(
		zap.String("Action", "ADD"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("Netns", args.Netns),
		zap.String("IfName", args.IfName),
	)

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
		HijackCIDR:       conf.ClusterCIDR,
		hostRuleTable:    *conf.HostRuleTable,
		ipFamily:         ipFamily,
		currentInterface: args.IfName,
		tuneMode:         conf.TuneMode,
		interfacePrefix:  conf.NICPrefix,
	}
	c.HijackCIDR = append(c.HijackCIDR, conf.ServiceCIDR...)
	c.HijackCIDR = append(c.HijackCIDR, conf.ExtraCIDR...)

	c.netns, err = ns.GetNS(args.Netns)
	if err != nil {
		logger.Error(err.Error())
		return fmt.Errorf("failed to GetNS %q: %v", args.Netns, err)
	}
	defer c.netns.Close()

	// check if it's first time invoke
	c.firstInvoke, err = firstInvoke(c.netns, conf.TuneMode)
	if err != nil {
		logger.Error(err.Error())
		return err
	}

	// get basic info
	switch conf.TuneMode {
	case ModeUnderlay:
		c.podVethName = defaultUnderlayVethName
		c.hostVethName = getHostVethName(args.ContainerID)
		if c.firstInvoke {
			c.hostVethHwAddress, c.podVethHwAddress, err = c.setupVeth(args.ContainerID)
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

		c.hostVethHwAddress, c.podVethHwAddress, err = networking.GetHwAddressByName(c.netns, defaultOverlayVethName)
		if err != nil {
			logger.Error("failed to get mac address in overlay mode", zap.Error(err))
			return fmt.Errorf("failed to get mac address in overlay mode: %v", err)
		}
	case ModeDisable:
		logger.Info("TuneMode is disable, nothing to do")
		return nil
	default:
		logger.Error("Unknown tuneMode", zap.String("invalid tuneMode", string(conf.TuneMode)))
		return fmt.Errorf("unknown tuneMode: %s", conf.TuneMode)
	}

	// we do check if ip is conflict firstly
	if conf.IPConflict != nil && conf.IPConflict.Enabled {
		err = ipchecking.DoIPConflictChecking(logger, c.netns, conf.IPConflict.Retry, conf.IPConflict.Interval, args.IfName, prevResult.IPs)
		if err != nil {
			logger.Error(err.Error())
			return err
		}
	}

	// overwrite mac address
	if len(conf.MacPrefix) != 0 {
		hwAddr, err := networking.OverwriteHwAddress(logger, c.netns, conf.MacPrefix, args.IfName)
		if err != nil {
			return fmt.Errorf("failed to update hardware address for interface %s, maybe hardware_prefix(%s) is invalid: %v", args.IfName, conf.MacPrefix, err)
		}

		logger.Info("Override hardware address successfully", zap.String("interface", args.IfName), zap.String("hardware address", hwAddr))
		if conf.OnlyHardware {
			logger.Debug("Only override hardware address, exit now")
			return types.PrintResult(conf.PrevResult, conf.CNIVersion)
		}
	}

	// get all ip address on the node
	c.hostAddress, err = networking.IPAddressOnNode(logger, ipFamily)
	if err != nil {
		logger.Error("failed to get IPAddressOnNode", zap.Error(err))
		return fmt.Errorf("failed to get IPAddressOnNode: %v", err)
	}

	// get ips of this interface(preInterfaceName) from, including ipv4 and ipv6
	c.currentAddress, err = networking.IPAddressByName(c.netns, args.IfName, ipFamily)
	if err != nil {
		logger.Error(err.Error())
		return fmt.Errorf("failed to IPAddressByName for pod %s : %v", args.IfName, err)
	}

	logger.Debug("Get currentAddress", zap.Any("currentAddress", c.currentAddress))

	if ipFamily != netlink.FAMILY_V4 {
		// ensure ipv6 is enable
		if err := sysctl.EnableIpv6Sysctl(c.netns); err != nil {
			logger.Error(err.Error())
			return err
		}
	}

	if conf.RPFilter != -1 {
		if err = sysctl.SysctlRPFilter(c.netns, conf.RPFilter); err != nil {
			logger.Error(err.Error())
			return err
		}
	}

	if err = c.setupNeighborhood(logger); err != nil {
		logger.Error("failed to setupNeighborhood", zap.Error(err))
		return err
	}

	c.currentRuleTable = unix.RT_TABLE_MAIN
	if !c.firstInvoke {
		c.currentRuleTable = c.getRuleNumber(c.currentInterface)
		if c.currentRuleTable < 0 {
			logger.Error("failed to getRuleNumber, maybe the pod's multus annotations doesn't match the tuneMode",
				zap.String("currentInterface", c.currentInterface), zap.String("interfacePrefix", c.interfacePrefix))
			return fmt.Errorf("failed to getRuleNumber, maybe the pod's multus annotations doesn't match the tuneMode")
		}

		if err = c.tunePodRoutes(logger, conf.PodDefaultRouteNIC); err != nil {
			logger.Error("failed to tunePodRoutes", zap.Error(err))
			return fmt.Errorf("failed to tunePodRoutes: %v", err)
		}
	}

	if err = c.setupRoutes(logger, c.currentRuleTable); err != nil {
		logger.Error(err.Error())
		return err
	}

	logger.Info("coordinator end", zap.Int64("Time Cost", time.Since(startTime).Microseconds()))
	return types.PrintResult(conf.PrevResult, conf.CNIVersion)
}
