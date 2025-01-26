// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	"go.uber.org/zap"

	agentOpenAPIClient "github.com/spidernet-io/spiderpool/api/v1/agent/client"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

// Set up file logging for spiderpool bin.
func SetupFileLogging(conf *NetConf) (*zap.Logger, error) {
	v := logutils.ConvertLogLevel(conf.IPAM.LogLevel)
	if v == nil {
		return nil, fmt.Errorf("unsupported log level %s", conf.IPAM.LogLevel)
	}

	return logutils.InitFileLogger(
		*v,
		conf.IPAM.LogFilePath,
		conf.IPAM.LogFileMaxSize,
		conf.IPAM.LogFileMaxAge,
		conf.IPAM.LogFileMaxCount,
	)
}

func deleteIpamIps(spiderpoolAgentAPI *agentOpenAPIClient.SpiderpoolAgentAPI, args *skel.CmdArgs, k8sArgs K8sArgs) error {
	_, err := spiderpoolAgentAPI.Daemonset.DeleteIpamIps(daemonset.NewDeleteIpamIpsParams().WithContext(context.TODO()).WithIpamBatchDelArgs(
		&models.IpamBatchDelArgs{
			ContainerID:  &args.ContainerID,
			NetNamespace: args.Netns,
			PodName:      (*string)(&k8sArgs.K8S_POD_NAME),
			PodNamespace: (*string)(&k8sArgs.K8S_POD_NAMESPACE),
			PodUID:       (*string)(&k8sArgs.K8S_POD_UID),
		},
	))
	return err
}
