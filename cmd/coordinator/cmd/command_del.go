// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"fmt"
	"net"
	"os"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	plugincmd "github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
	"github.com/spidernet-io/spiderpool/pkg/openapi"
)

func CmdDel(args *skel.CmdArgs) (err error) {
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
	), withCNICommand(cniCommandDel))
	if err != nil {
		return fmt.Errorf("failed to GetCoordinatorConfig: %w", err)
	}
	coordinatorConfig := resp.Payload

	conf, err := ParseConfig(args.StdinData, coordinatorConfig)
	if err != nil {
		return err
	}

	if conf.Mode == ModeDisable {
		return nil
	}

	logger, err := logutils.SetupFileLogging(conf.LogOptions.LogLevel,
		conf.LogOptions.LogFilePath, conf.LogOptions.LogFileMaxSize,
		conf.LogOptions.LogFileMaxAge, conf.LogOptions.LogFileMaxCount)
	if err != nil {
		return fmt.Errorf("failed to init logger: %w ", err)
	}

	logger = logger.Named(BinNamePlugin).With(
		zap.String("Action", "DELETE"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("Netns", args.Netns),
		zap.String("IfName", args.IfName),
	)

	logger.Info(fmt.Sprintf("start to implement DELETE command in %v mode", conf.Mode))

	c := &coordinator{
		hostRuleTable: int(*conf.HostRuleTable),
	}

	hostVeth := getHostVethName(args.ContainerID)
	vethLink, err := netlink.LinkByName(hostVeth)
	if err != nil {
		var linkNotFoundErr netlink.LinkNotFoundError
		if errors.As(err, &linkNotFoundErr) {
			logger.Sugar().Debug("Host veth has gone, nothing to do", zap.String("HostVeth", hostVeth))
		} else {
			logger.Warn("failed to get host veth; continue with remaining cleanup",
				zap.String("HostVeth", hostVeth), zap.Error(err))
		}
	} else {
		filterRoute := &netlink.Route{
			LinkIndex: vethLink.Attrs().Index,
			Table:     c.hostRuleTable,
		}
		routes, routeListErr := netlink.RouteListFiltered(netlink.FAMILY_ALL, filterRoute, netlink.RT_FILTER_TABLE|netlink.RT_FILTER_OIF)
		if routeListErr != nil {
			logger.Warn("failed to list routes for host veth; continue with remaining cleanup",
				zap.String("HostVeth", hostVeth), zap.Int("HostRuleTable", c.hostRuleTable), zap.Error(routeListErr))
		}

		for idx := range routes {
			route := routes[idx]
			if err = netlink.RouteDel(&route); err != nil && !os.IsNotExist(err) {
				logger.Warn("failed to delete route for host veth; continue with remaining cleanup",
					zap.String("HostVeth", hostVeth), zap.String("Route", route.String()), zap.Error(err))
			} else {
				logger.Debug("deleted route for host veth", zap.String("HostVeth", hostVeth), zap.String("Route", route.String()))
			}

			// Older Coordinator versions created a per-Pod `to <PodIP>` rule.
			// Current ADD uses one shared rule, which must not be removed by DEL.
			deleteLegacyHostRule(logger, route.Dst, c.hostRuleTable)
		}

		if err = netlink.LinkDel(vethLink); err != nil && !os.IsNotExist(err) {
			var linkNotFoundErr netlink.LinkNotFoundError
			if !errors.As(err, &linkNotFoundErr) {
				logger.Warn("failed to delete host veth; continue with remaining cleanup",
					zap.String("HostVeth", hostVeth), zap.Error(err))
			}
		} else {
			logger.Debug("deleted host veth", zap.String("HostVeth", hostVeth))
		}
	}

	c.netns, err = ns.GetNS(args.Netns)
	if err != nil {
		var nsPathErr ns.NSPathNotExistErr
		if errors.As(err, &nsPathErr) {
			logger.Debug("Pod's netns already gone; skipped netns cleanup")
			logger.Info("cmdDel end")
			return nil
		}
		logger.Warn("failed to get Pod netns; skipped netns cleanup", zap.String("Netns", args.Netns), zap.Error(err))
		logger.Info("cmdDel end")
		return nil
	}
	defer func() { _ = c.netns.Close() }()

	err = c.netns.Do(func(netNS ns.NetNS) error {
		c.currentAddress, err = networking.GetAddersByName(args.IfName, netlink.FAMILY_ALL)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		logger.Warn("failed to get interface addresses; skipped legacy rule cleanup", zap.String("IfName", args.IfName), zap.Error(err))
	}

	for idx := range c.currentAddress {
		ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)
		deleteLegacyHostRule(logger, ipNet, c.hostRuleTable)
	}

	logger.Info("cmdDel end")
	return nil
}

func deleteLegacyHostRule(logger *zap.Logger, dst *net.IPNet, hostRuleTable int) {
	if dst == nil {
		return
	}

	if err := networking.DelToRuleTable(dst, hostRuleTable); err != nil && !os.IsNotExist(err) {
		logger.Warn("failed to delete legacy per-Pod host rule; continue with remaining cleanup",
			zap.Int("HostRuleTable", hostRuleTable), zap.String("Dst", dst.String()), zap.Error(err))
	} else {
		logger.Debug("deleted legacy per-Pod host rule", zap.Int("HostRuleTable", hostRuleTable), zap.String("Dst", dst.String()))
	}
}
