// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	plugincmd "github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
	"os"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
)

func CmdDel(args *skel.CmdArgs) (err error) {
	k8sArgs := plugincmd.K8sArgs{}
	if err = types.LoadArgs(args.Args, &k8sArgs); nil != err {
		return fmt.Errorf("failed to load CNI ENV args: %w", err)
	}

	client, err := cmd.NewAgentOpenAPIUnixClient(constant.DefaultIPAMUnixSocketPath)
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
	logger.Info(fmt.Sprintf("start to implement DELETE command in %v mode", conf.TuneMode))

	c := &coordinator{
		hostRuleTable: int(*conf.HostRuleTable),
	}

	c.netns, err = ns.GetNS(args.Netns)
	if err != nil {
		_, ok := err.(ns.NSPathNotExistErr)
		if ok {
			logger.Debug("Pod's netns already gone.  Nothing to do.")
		} else {
			logger.Warn("failed to GetNS, container maybe gone, ignore ", zap.Error(err))
		}
	} else {
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
			logger.Warn("failed to GetAddersByName, ignore error", zap.Error(err))
		} else {
			for idx := range c.currentAddress {
				ipNet := networking.ConvertMaxMaskIPNet(c.currentAddress[idx].IP)
				err = networking.DelToRuleTable(ipNet, c.hostRuleTable)
				if err != nil && !os.IsNotExist(err) {
					logger.Error("failed to DelToRuleTable", zap.Int("HostRuleTable", c.hostRuleTable), zap.String("Dst", ipNet.String()), zap.Error(err))
				}
			}
		}
	}

	if conf.TuneMode == ModeUnderlay {
		hostVeth := getHostVethName(args.ContainerID)
		vethLink, err := netlink.LinkByName(hostVeth)
		if err != nil {
			if _, ok := err.(netlink.LinkNotFoundError); ok {
				logger.Debug("Host veth has gone, nothing to do", zap.String("HostVeth", hostVeth))
			} else {
				logger.Warn(fmt.Sprintf("failed to get host veth device %s: %v", hostVeth, err))
				return fmt.Errorf("failed to get host veth device %s: %v", hostVeth, err)
			}
		} else {
			if err = netlink.LinkDel(vethLink); err != nil {
				logger.Warn("failed to del hostVeth", zap.Error(err))
				return fmt.Errorf("failed to del hostVeth %s: %w", hostVeth, err)
			} else {
				logger.Debug("success to del hostVeth", zap.String("HostVeth", hostVeth))
			}
		}
	}

	logger.Info("cmdDel end")
	return nil
}
