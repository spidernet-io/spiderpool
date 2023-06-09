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
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	plugincmd "github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"github.com/spidernet-io/spiderpool/internal/version"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/gwconnection"
	"github.com/spidernet-io/spiderpool/pkg/networking/ipchecking"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/spidernet-io/spiderpool/pkg/networking/sysctl"
)

func CmdAdd(args *skel.CmdArgs) (err error) {
	startTime := time.Now()

	client, err := cmd.NewAgentOpenAPIUnixClient(constant.DefaultIPAMUnixSocketPath)
	if err != nil {
		return err
	}

	resp, err := client.Daemonset.GetCoordinatorConfig(daemonset.NewGetCoordinatorConfigParams())
	if err != nil {
		return fmt.Errorf("failed to GetCoordinatorConfig: %v", err)
	}
	coordinatorConfig := resp.Payload

	conf, err := ParseConfig(args.StdinData, coordinatorConfig)
	if err != nil {
		return err
	}

	logger, err := logutils.SetupFileLogging(conf.LogOptions.LogLevel,
		conf.LogOptions.LogFilePath, conf.LogOptions.LogFileMaxSize,
		conf.LogOptions.LogFileMaxAge, conf.LogOptions.LogFileMaxCount)
	if err != nil {
		return fmt.Errorf("failed to init logger: %v ", err)
	}

	logger.Info("coordinator starting", zap.String("Version", version.CoordinatorBuildDateVersion()),
		zap.String("Branch", version.CoordinatorGitBranch()),
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

	k8sArgs := plugincmd.K8sArgs{}
	if err = types.LoadArgs(args.Args, &k8sArgs); nil != err {
		err := fmt.Errorf("failed to load CNI ENV args: %w", err)
		logger.Error(err.Error())
		return err
	}

	logger = logger.With(
		zap.String("PodName", string(k8sArgs.K8S_POD_NAME)),
		zap.String("PodNamespace", string(k8sArgs.K8S_POD_NAMESPACE)),
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
		hostRuleTable:    int(*conf.HostRuleTable),
		ipFamily:         ipFamily,
		currentInterface: args.IfName,
		tuneMode:         conf.TuneMode,
		interfacePrefix:  conf.InterfacePrefix,
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
	err = c.coordinatorFirstInvoke(conf.PodFirstInterface)
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
			err = c.setupVeth(args.ContainerID)
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
		logger.Error("Unknown tuneMode", zap.String("invalid tuneMode", string(conf.TuneMode)))
		return fmt.Errorf("unknown tuneMode: %s", conf.TuneMode)
	}

	logger.Sugar().Infof("Get coordinator config: %v", c)

	//  we do detect gateway connection firstly
	if *conf.DetectGateway {
		logger.Debug("Try to detect gateway")

		var gws []string
		err = c.netns.Do(func(netNS ns.NetNS) error {
			gws, err = networking.GetDefaultGatewayByName(c.currentInterface, c.ipFamily)
			if err != nil {
				logger.Error("failed to GetDefaultGatewayByName", zap.Error(err))
				return fmt.Errorf("failed to GetDefaultGatewayByName: %v", err)
			}
			return nil
		})
		if err != nil {
			return err
		}

		logger.Debug("Get GetDefaultGatewayByName", zap.Strings("Gws", gws))

		for _, gw := range gws {
			if err = gwconnection.DetectGatewayConnection(gw); err != nil {
				logger.Error(err.Error())
				return err
			}
		}

		if err != nil {
			return err
		}
		logger.Debug("Success to detect gateway", zap.Strings("Gws", gws))
	}

	if conf.IPConflict != nil && conf.IPConflict.Enabled {
		err = ipchecking.DoIPConflictChecking(logger, c.netns, conf.IPConflict.Retry, conf.IPConflict.Interval, args.IfName, prevResult.IPs)
		if err != nil {
			logger.Error(err.Error())
			return fmt.Errorf("failed to check ip conflict: %w", err)
		}
		logger.Debug("Success to check IP conflict")
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

	c.currentRuleTable = c.getRuleNumber(c.currentInterface)
	if c.currentRuleTable < 0 {
		logger.Error("failed to getRuleNumber, maybe the pod's multus annotations doesn't match the tuneMode",
			zap.String("currentInterface", c.currentInterface), zap.String("interfacePrefix", c.interfacePrefix))
		return fmt.Errorf("failed to getRuleNumber, maybe the pod's multus annotations doesn't match the tuneMode")
	}

	if err = c.setupHostRoutes(logger); err != nil {
		logger.Error(err.Error())
		return err
	}

	if err = c.setupHijackRoutes(logger, c.currentRuleTable); err != nil {
		logger.Error("failed to setupHijackRoutes", zap.Error(err))
		return fmt.Errorf("failed to setupHijackRoutes: %v", err)
	}

	if conf.TunePodRoutes != nil && *conf.TunePodRoutes && (!c.firstInvoke || c.tuneMode == ModeOverlay) {
		logger.Debug("Try to tune pod routes")
		if err = c.tunePodRoutes(logger, conf.PodDefaultRouteNIC); err != nil {
			logger.Error("failed to tunePodRoutes", zap.Error(err))
			return fmt.Errorf("failed to tunePodRoutes: %v", err)
		}
		logger.Debug("Success to tune pod routes")
	}

	logger.Sugar().Infof("coordinator end, time cost: %v", time.Since(startTime))
	return types.PrintResult(conf.PrevResult, conf.CNIVersion)
}
