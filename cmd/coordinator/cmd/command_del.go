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
	"golang.org/x/sys/unix"

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
	))
	if err != nil {
		return fmt.Errorf("failed to GetCoordinatorConfig: %v", err)
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
		return fmt.Errorf("failed to init logger: %v ", err)
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

	// The kernel removes routes that reference the veth when unregistering it.
	hostVeth := getHostVethName(args.ContainerID)
	vethLink := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostVeth,
		},
	}
	if err = netlink.LinkDel(vethLink); err != nil {
		var linkNotFoundErr netlink.LinkNotFoundError
		if os.IsNotExist(err) || errors.Is(err, unix.ENODEV) || errors.As(err, &linkNotFoundErr) {
			logger.Debug("Host veth is already gone", zap.String("HostVeth", hostVeth))
		} else {
			logger.Warn("failed to delete host veth, continuing with remaining cleanup",
				zap.String("HostVeth", hostVeth), zap.Error(err))
		}
	} else {
		logger.Debug("deleted host veth", zap.String("HostVeth", hostVeth))
	}

	c.netns, err = ns.GetNS(args.Netns)
	if err != nil {
		var nsPathErr ns.NSPathNotExistErr
		if errors.As(err, &nsPathErr) {
			logger.Debug("Pod netns is already gone, skipping netns cleanup")
			logger.Info("cmdDel end")
			return nil
		}
		logger.Warn("failed to get Pod netns, skipping netns cleanup", zap.String("Netns", args.Netns), zap.Error(err))
		logger.Info("cmdDel end")
		return nil
	}
	defer c.netns.Close()

	err = c.netns.Do(func(netNS ns.NetNS) error {
		c.currentAddress, err = networking.GetAddersByName(args.IfName, netlink.FAMILY_ALL)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		logger.Warn("failed to get interface addresses, skipping legacy rule cleanup", zap.String("IfName", args.IfName), zap.Error(err))
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
		logger.Warn("failed to delete legacy per-Pod host rule, continuing with remaining cleanup",
			zap.Int("HostRuleTable", hostRuleTable), zap.String("Dst", dst.String()), zap.Error(err))
	} else {
		logger.Debug("deleted legacy per-Pod host rule", zap.Int("HostRuleTable", hostRuleTable), zap.String("Dst", dst.String()))
	}
}
