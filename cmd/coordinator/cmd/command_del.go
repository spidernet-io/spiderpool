// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	"github.com/spidernet-io/spiderpool/internal/version"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func CmdDel(args *skel.CmdArgs) (err error) {
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

	logger.Info("coordinator cmdDel starting", zap.String("Version", version.CoordinatorBuildDateVersion()), zap.String("Branch", version.CoordinatorGitBranch()),
		zap.String("Commit", version.CoordinatorGitCommit()),
		zap.String("Build time", version.CoordinatorBuildDate()),
		zap.String("Go Version", version.GoString()))

	logger = logger.Named(BinNamePlugin).With(
		zap.String("Action", "ADD"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("Netns", args.Netns),
		zap.String("IfName", args.IfName),
	)

	c := &coordinator{
		hostRuleTable: int(*conf.HostRuleTable),
	}

	c.netns, err = ns.GetNS(args.Netns)
	if err != nil {
		_, ok := err.(ns.NSPathNotExistErr)
		if ok {
			logger.Debug("Pod's netns already gone.  Nothing to do.")
			return nil
		}
		logger.Error("failed to GetNS", zap.Error(err))
		return err
	}
	defer c.netns.Close()

	err = c.netns.Do(func(netNS ns.NetNS) error {
		c.currentAddress, err = networking.GetAddersByName(args.IfName, netlink.FAMILY_ALL)
		if err != nil {
			logger.Error("failed to GetAddersByName", zap.String("interface", args.IfName))
			return err
		}
		return nil
	})

	if err != nil {
		// ignore err
		logger.Error("failed to GetAddersByName, ignore error")
		return nil
	}

	for idx := range c.currentAddress {
		ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)
		err = networking.DelToRuleTable(ipNet, c.hostRuleTable)
		if err != nil && !os.IsNotExist(err) {
			logger.Error("failed to DelToRuleTable", zap.Int("HostRuleTable", c.hostRuleTable), zap.String("Dst", ipNet.String()), zap.Error(err))
			return fmt.Errorf("failed to DelToRuleTable: %v", err)
		}
	}
	logger.Info("coordinator cmdDel end")
	return nil
}
