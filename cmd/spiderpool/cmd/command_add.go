// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	current "github.com/containernetworking/cni/pkg/types/100"
	"github.com/go-openapi/strfmt"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/cmd/spiderpool-agent/cmd"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
)

var (
	BinNamePlugin       = filepath.Base(os.Args[0])
	ErrAgentHealthCheck = fmt.Errorf("err: get spiderpool agent health check failed")
	ErrPostIPAM         = fmt.Errorf("err: get ipam allocation failed")
)

// CmdAdd follows CNI SPEC cmdAdd.
func CmdAdd(args *skel.CmdArgs) (err error) {
	var logger *zap.Logger

	// Defer a panic recover, so that in case we panic we can still return
	// a proper error to the runtime.
	defer func() {
		if e := recover(); e != nil {
			msg := fmt.Sprintf("Spiderpool IPAM CNI panicked during ADD: %s", e)
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

	err = setupFileLogging(conf)
	if nil != err {
		return fmt.Errorf("Unable to setup logging: %w", err)
	}

	// new cmdAdd logger
	logger = logutils.LoggerFile.Named(BinNamePlugin)
	logger.Sugar().Debugf("Processing CNI ADD request %#v", args)
	logger.Sugar().Debugf("CNI ADD NetConf: %#v", conf)

	k8sArgs := K8sArgs{}
	if err = types.LoadArgs(args.Args, &k8sArgs); nil != err {
		logger.Error(err.Error(), zap.String("Action", "Add"), zap.String("ContainerID", args.ContainerID))
		return err
	}
	logger.Sugar().Debugf("CNI ADD Args: %#v", k8sArgs)

	// register some args into logger
	logger = logger.With(zap.String("Action", "Add"),
		zap.String("ContainerID", args.ContainerID),
		zap.String("PodUID", string(k8sArgs.K8S_POD_UID)),
		zap.String("PodName", string(k8sArgs.K8S_POD_NAME)),
		zap.String("PodNamespace", string(k8sArgs.K8S_POD_NAMESPACE)),
		zap.String("IfName", args.IfName))
	logger.Info("Generate IPAM configuration")

	// new unix client
	spiderpoolAgentAPI, err := cmd.NewAgentOpenAPIUnixClient(conf.IpamUnixSocketPath)
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

	// POST /ipam/ip
	logger.Info("Sending IP assignment request to spider agent.")
	ipamAddArgs := &models.IpamAddArgs{
		ContainerID:  &args.ContainerID,
		IfName:       &args.IfName,
		NetNamespace: &args.Netns,
		PodName:      (*string)(&k8sArgs.K8S_POD_NAME),
		PodNamespace: (*string)(&k8sArgs.K8S_POD_NAMESPACE),
	}

	params := daemonset.NewPostIpamIPParams()
	params.SetIpamAddArgs(ipamAddArgs)
	ipamResponse, err := spiderpoolAgentAPI.Daemonset.PostIpamIP(params)
	if nil != err {
		logger.Error(err.Error())
		return ErrPostIPAM
	}
	// validate spiderpool-agent response
	if err = ipamResponse.Payload.Validate(strfmt.Default); nil != err {
		logger.Error(err.Error())
		return err
	}

	// assemble result with ipam response.
	result := assembleResult(conf.CNIVersion, args.IfName, ipamResponse)

	logger.Sugar().Infof("IPAM assigned successfully: %s", result.IPs)

	return types.PrintResult(result, conf.CNIVersion)
}

// assembleResult combines the cni result with spiderpool agent response.
func assembleResult(cniVersion, IfName string, ipamResponse *daemonset.PostIpamIPOK) *current.Result {
	// IPAM Plugin Result
	result := &current.Result{
		CNIVersion: cniVersion,
	}

	// Result DNS
	if nil != ipamResponse.Payload.DNS {
		result.DNS = types.DNS{
			Nameservers: ipamResponse.Payload.DNS.Nameservers,
			Domain:      ipamResponse.Payload.DNS.Domain,
			Search:      ipamResponse.Payload.DNS.Search,
			Options:     ipamResponse.Payload.DNS.Options,
		}
	}

	// Result Routes
	var routes []*types.Route
	for _, singleRoute := range ipamResponse.Payload.Routes {
		var route types.Route

		routeDstAddr := net.ParseIP(*singleRoute.Dst)
		routeDst := net.IPNet{IP: routeDstAddr}
		if routeDstAddr.To4() == nil {
			// ipv6 address
			routeDst.Mask = net.CIDRMask(128, 128)
		} else {
			// ipv4 address
			routeDst.Mask = net.CIDRMask(32, 32)
		}
		route.Dst = routeDst
		route.GW = net.ParseIP(*singleRoute.Gw)

		routes = append(routes, &route)
	}
	result.Routes = routes

	// Result Interfaces, IPs
	var netInterfaces []*current.Interface
	// for NIC index recording.
	tmpIndex := 0
	for _, ipconfig := range ipamResponse.Payload.Ips {
		// filter IPAM multi-Interfaces
		if *ipconfig.Nic == IfName {
			nic := &current.Interface{Name: *ipconfig.Nic}
			netInterfaces = append(netInterfaces, nic)

			// record ips
			if *ipconfig.Version == 4 {
				var v4ip current.IPConfig
				nicIndex := tmpIndex

				v4ip.Interface = &nicIndex
				v4ip.Address = net.IPNet{IP: net.ParseIP(*ipconfig.Address), Mask: net.CIDRMask(32, 32)}
				v4ip.Gateway = net.ParseIP(ipconfig.Gateway)

				result.IPs = append(result.IPs, &v4ip)
			} else {
				var v6ip current.IPConfig
				nicIndex := tmpIndex

				v6ip.Interface = &nicIndex
				v6ip.Address = net.IPNet{IP: net.ParseIP(*ipconfig.Address), Mask: net.CIDRMask(128, 128)}
				v6ip.Gateway = net.ParseIP(ipconfig.Gateway)

				result.IPs = append(result.IPs, &v6ip)
			}
			tmpIndex++
		}
	}
	result.Interfaces = netInterfaces

	return result
}
