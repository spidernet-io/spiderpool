// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"runtime/debug"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/openapi"
)

// CmdDel follows CNI SPEC cmdDel.
func CmdDel(args *skel.CmdArgs) (err error) {
	var logger *zap.Logger

	// Defer a panic recover, so that in case we panic we can still return
	// a proper error to the runtime.
	defer func() {
		if e := recover(); e != nil {
			msg := fmt.Sprintf("Spiderpool IPAM CNI panicked during DEL: %v", e)

			if err != nil {
				// If it is recovering and an error occurs, then we need to
				// present both.
				msg = fmt.Sprintf("%s: error=%v", msg, err.Error())
			}

			if nil != logger {
				logger.Sugar().Errorf("%s\n\n%s", msg, debug.Stack())
			}
		}
	}()

	conf, err := LoadNetConf(args.StdinData)
	if nil != err {
		return fmt.Errorf("failed to load CNI network configuration: %w", err)
	}

	logger, err = SetupFileLogging(conf)
	if nil != err {
		return fmt.Errorf("failed to setup file logging: %w", err)
	}

	logger = logger.Named(BinNamePlugin).With(
		zap.String("Action", "DEL"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("Netns", args.Netns),
		zap.String("IfName", args.IfName),
	)
	logger.Debug("Processing CNI DEL request")
	logger.Sugar().Debugf("CNI network configuration: %+v", *conf)

	k8sArgs := K8sArgs{}
	if err = types.LoadArgs(args.Args, &k8sArgs); nil != err {
		err := fmt.Errorf("failed to load CNI ENV args: %w", err)
		logger.Error(err.Error())
		return err
	}

	logger = logger.With(
		zap.String("PodName", string(k8sArgs.K8S_POD_NAME)),
		zap.String("PodNamespace", string(k8sArgs.K8S_POD_NAMESPACE)),
		zap.String("PodUID", string(k8sArgs.K8S_POD_UID)),
	)
	logger.Sugar().Debugf("CNI ENV args: %+v", k8sArgs)

	spiderpoolAgentAPI, err := openapi.NewAgentOpenAPIUnixClient(conf.IPAM.IPAMUnixSocketPath)
	if nil != err {
		err := fmt.Errorf("failed to create spiderpool-agent client: %w", err)
		logger.Error(err.Error())
		return err
	}

	logger.Debug("Send health check request to spiderpool-agent backend")
	_, err = spiderpoolAgentAPI.Connectivity.GetIpamHealthy(connectivity.NewGetIpamHealthyParams())
	if nil != err {
		err := fmt.Errorf("%w, failed to check: %w", ErrAgentHealthCheck, err)
		logger.Error(err.Error())
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	params := daemonset.NewDeleteIpamIPParams().
		WithContext(ctx).
		WithIpamDelArgs(&models.IpamDelArgs{
			ContainerID:  &args.ContainerID,
			NetNamespace: args.Netns,
			IfName:       &args.IfName,
			PodNamespace: (*string)(&k8sArgs.K8S_POD_NAMESPACE),
			PodName:      (*string)(&k8sArgs.K8S_POD_NAME),
			PodUID:       (*string)(&k8sArgs.K8S_POD_UID),
		})

	logger.Debug("Send IPAM request")
	_, err = spiderpoolAgentAPI.Daemonset.DeleteIpamIP(params)
	if nil != err {
		logger.Sugar().Errorf("%v: %w", ErrDeleteIPAM, err)
		return nil
	}

	logger.Info("IPAM release successfully")
	return nil
}
