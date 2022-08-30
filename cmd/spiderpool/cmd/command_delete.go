// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	"go.uber.org/zap"
)

var ErrDeleteIPAM = fmt.Errorf("err: get ipam release failed")

// CmdDel follows CNI SPEC cmdDel.
func CmdDel(args *skel.CmdArgs) (err error) {
	var logger *zap.Logger

	// Defer a panic recover, so that in case we panic we can still return
	// a proper error to the runtime.
	defer func() {
		if e := recover(); e != nil {
			msg := fmt.Sprintf("Spiderpool IPAM CNI panicked during DEL: %s", e)
			if err != nil {
				// If we're recovering and there was also an error, then we need to
				// present both.
				msg = fmt.Sprintf("%s: error=%s", msg, err)
			}
			if nil != logger {
				logger.Error(msg)
			}

			err = fmt.Errorf(msg)
		}
	}()

	conf, err := LoadNetConf(args.StdinData)
	if nil != err {
		return fmt.Errorf("Load network config failed: %v", err)
	}

	logger, err = setupFileLogging(conf)
	if nil != err {
		return fmt.Errorf("Unable to setup logging: %w", err)
	}

	// new cmdDel logger
	logger = logger.Named(BinNamePlugin)
	logger.Sugar().Debugf("Processing CNI DEL request %+v", args)
	logger.Sugar().Debugf("CNI DEL NetConf: %+v", conf)

	k8sArgs := K8sArgs{}
	if err = types.LoadArgs(args.Args, &k8sArgs); nil != err {
		logger.Error(err.Error(), zap.String("Action", "Del"), zap.String("ContainerID", args.ContainerID))
		return err
	}
	logger.Sugar().Debugf("CNI DEL Args: %#v", k8sArgs)

	// register some args into logger
	logger = logger.With(zap.String("Action", "Del"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("PodUID", string(k8sArgs.K8S_POD_UID)),
		zap.String("PodName", string(k8sArgs.K8S_POD_NAME)),
		zap.String("PodNamespace", string(k8sArgs.K8S_POD_NAMESPACE)),
		zap.String("IfName", args.IfName))
	logger.Info("Generate IPAM configuration")

	// new unix client
	spiderpoolAgentAPI, err := cmd.NewAgentOpenAPIUnixClient(conf.IPAM.IpamUnixSocketPath)
	if nil != err {
		logger.Error(err.Error())
		return err
	}

	// GET /ipam/healthy
	logger.Debug("Sending health check to spider agent.")
	_, err = spiderpoolAgentAPI.Connectivity.GetIpamHealthy(connectivity.NewGetIpamHealthyParams())
	if nil != err {
		logger.Error(err.Error())
		return ErrAgentHealthCheck
	}
	logger.Debug("Health check succeed.")

	// DELETE /ipam/ip
	logger.Info("Sending IP release request to spider agent.")
	ipamDelArgs := &models.IpamDelArgs{
		ContainerID:  &args.ContainerID,
		IfName:       &args.IfName,
		NetNamespace: args.Netns,
		PodName:      (*string)(&k8sArgs.K8S_POD_NAME),
		PodNamespace: (*string)(&k8sArgs.K8S_POD_NAMESPACE),
	}

	params := daemonset.NewDeleteIpamIPParams()
	params.SetIpamDelArgs(ipamDelArgs)
	_, err = spiderpoolAgentAPI.Daemonset.DeleteIpamIP(params)
	if nil != err {
		logger.Error(err.Error())
		return ErrDeleteIPAM
	}

	logger.Info("IP release successfully.")
	return nil
}
