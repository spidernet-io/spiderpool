// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net"
	"runtime/debug"
	"time"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/go-openapi/strfmt"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/openapi"
)

// CmdAdd follows CNI SPEC cmdAdd.
func CmdAdd(args *skel.CmdArgs) (err error) {
	var logger *zap.Logger

	// Defer a panic recover, so that in case we panic we can still return
	// a proper error to the runtime.
	defer func() {
		if e := recover(); e != nil {
			msg := fmt.Sprintf("Spiderpool IPAM CNI panicked during ADD: %v", e)

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
		return fmt.Errorf("failed to load CNI network configuration: %v", err)
	}

	logger, err = setupFileLogging(conf)
	if nil != err {
		return fmt.Errorf("failed to setup file logging: %v", err)
	}

	logger = logger.Named(BinNamePlugin).With(
		zap.String("Action", "ADD"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("Netns", args.Netns),
		zap.String("IfName", args.IfName),
	)
	logger.Debug("Processing CNI ADD request")
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
		err := fmt.Errorf("%w, failed to check: %v", ErrAgentHealthCheck, err)
		logger.Error(err.Error())
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()

	params := daemonset.NewPostIpamIPParams().
		WithContext(ctx).
		WithIpamAddArgs(&models.IpamAddArgs{
			ContainerID:       &args.ContainerID,
			NetNamespace:      &args.Netns,
			IfName:            &args.IfName,
			PodName:           (*string)(&k8sArgs.K8S_POD_NAME),
			PodNamespace:      (*string)(&k8sArgs.K8S_POD_NAMESPACE),
			PodUID:            (*string)(&k8sArgs.K8S_POD_UID),
			DefaultIPV4IPPool: conf.IPAM.DefaultIPv4IPPool,
			DefaultIPV6IPPool: conf.IPAM.DefaultIPv6IPPool,
			CleanGateway:      conf.IPAM.CleanGateway,
		})

	logger.Debug("Send IPAM request")
	ipamResponse, err := spiderpoolAgentAPI.Daemonset.PostIpamIP(params)
	if nil != err {
		err := fmt.Errorf("%w: %v", ErrPostIPAM, err)
		logger.Error(err.Error())
		return err
	}

	// Validate IPAM request response.
	if err = ipamResponse.Payload.Validate(strfmt.Default); nil != err {
		err := fmt.Errorf("%w: %v", ErrPostIPAM, err)
		logger.Error(err.Error())
		return err
	}

	// Assemble the result of IPAM request response.
	result, err := assembleResult(conf.CNIVersion, args.IfName, ipamResponse)
	if err != nil {
		err := fmt.Errorf("%w: %v", ErrPostIPAM, err)
		logger.Error(err.Error())
		return err
	}

	logger.Sugar().Infof("IPAM allocation result: %+v", *result)
	return types.PrintResult(result, conf.CNIVersion)
}

// assembleResult groups the IP allocation resutls of IPAM request response
// based on NIC and combines them into CNI results.
func assembleResult(cniVersion, IfName string, ipamResponse *daemonset.PostIpamIPOK) (*current.Result, error) {
	result := &current.Result{
		CNIVersion: cniVersion,
	}

	// Mock DNS.
	if nil != ipamResponse.Payload.DNS {
		result.DNS = types.DNS{
			Nameservers: ipamResponse.Payload.DNS.Nameservers,
			Domain:      ipamResponse.Payload.DNS.Domain,
			Search:      ipamResponse.Payload.DNS.Search,
			Options:     ipamResponse.Payload.DNS.Options,
		}
	}

	var routes []*types.Route
	for _, route := range ipamResponse.Payload.Routes {
		if *route.IfName == IfName {
			_, dst, err := net.ParseCIDR(*route.Dst)
			if err != nil {
				return nil, err
			}
			routes = append(routes, &types.Route{
				Dst: *dst,
				GW:  net.ParseIP(*route.Gw),
			})
		}
	}
	result.Routes = routes

	for _, ip := range ipamResponse.Payload.Ips {
		if *ip.Nic == IfName {
			address, err := spiderpoolip.ParseIP(*ip.Version, *ip.Address, true)
			if err != nil {
				return nil, err
			}
			result.IPs = append(result.IPs, &current.IPConfig{
				Address: *address,
				Gateway: net.ParseIP(ip.Gateway),
			})
		}
	}

	if len(result.IPs) == 0 {
		return nil, fmt.Errorf("no Interface %s IP allocation found", IfName)
	}

	return result, nil
}
