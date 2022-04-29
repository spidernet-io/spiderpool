// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"encoding/json"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	"github.com/spidernet-io/spiderpool/pkg/cnicommon"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
)

// CmdDel follows CNI SPEC cmdDel.
func CmdDel(args *skel.CmdArgs) error {
	// new cmdDel logger
	logger := logutils.LoggerFile.Named(BinNamePlugin)

	conf := types.NetConf{}
	if err := json.Unmarshal(args.StdinData, &conf); nil != err {
		logger.Error(err.Error(), zap.String("Action", "Del"), zap.String("ContainerID", args.ContainerID))
		return err
	}

	k8sArgs := cnicommon.K8sArgs{}
	if err := types.LoadArgs(args.Args, &k8sArgs); nil != err {
		logger.Error(err.Error(), zap.String("Action", "Del"), zap.String("ContainerID", args.ContainerID))
		return err
	}

	// register some args into logger
	logger = logger.With(zap.String("Action", "Del"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("PodUID", string(k8sArgs.K8S_POD_UID)),
		zap.String("PodName", string(k8sArgs.K8S_POD_NAME)),
		zap.String("PodNamespace", string(k8sArgs.K8S_POD_NAMESPACE)))
	logger.Debug("Generate IPAM configuration")

	// new unix client
	spiderpoolAgentAPI := cmd.NewAgentOpenAPIUnixClient()

	// GET /ipam/healthy
	_, err := spiderpoolAgentAPI.Connectivity.GetIpamHealthy(connectivity.NewGetIpamHealthyParams())
	if nil != err {
		logger.Error(err.Error())
		return err
	}

	// DELETE /ipam/ip
	_, err = spiderpoolAgentAPI.Daemonset.DeleteIpamIP(daemonset.NewDeleteIpamIPParams())
	if nil != err {
		logger.Error(err.Error())
		return err
	}

	return nil
}
