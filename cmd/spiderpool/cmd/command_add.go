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
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/go-openapi/strfmt"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/client/connectivity"
	"github.com/spidernet-io/spiderpool/api/v1/agent/client/daemonset"
	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/networking/networking"
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

			if logger != nil {
				logger.Sugar().Errorf("%s\n\n%s", msg, debug.Stack())
			}
		}
	}()

	conf, err := LoadNetConf(args.StdinData)
	if nil != err {
		return fmt.Errorf("failed to load CNI network configuration: %v", err)
	}

	netns, err := ns.GetNS(args.Netns)
	if err != nil {
		return fmt.Errorf("failed to GetNS %q for pod: %v", args.Netns, err)
	}
	defer netns.Close()

	logger, err = SetupFileLogging(conf)
	if err != nil {
		return fmt.Errorf("failed to setup file logging: %v", err)
	}

	// When IPAM is invoked, the NIC is down and must be set it up in order to detect IP conflicts and
	// gateway reachability.
	err = netns.Do(func(netNS ns.NetNS) error {
		l, err := netlink.LinkByName(args.IfName)
		if err != nil {
			return fmt.Errorf("failed to get link: %w", err)
		}

		if err = netlink.LinkSetUp(l); err != nil {
			return fmt.Errorf("failed to set link up: %w", err)
		}

		logger.Sugar().Debugf("Set link %s to up for IP conflict and gateway detection", args.IfName)
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to set link up: %w", err)
	}

	hostNs, err := ns.GetCurrentNS()
	if err != nil {
		return fmt.Errorf("failed to get current netns: %v", err)
	}
	defer hostNs.Close()

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
	if err != nil {
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

	// do ip conflict and gateway detection
	logger.Sugar().Info("postIpam response",
		zap.Any("DNS", ipamResponse.Payload.DNS),
		zap.Any("Routes", ipamResponse.Payload.Routes))

	if err = DetectIPConflictAndGatewayReachable(logger, args.IfName, hostNs, netns, ipamResponse.Payload.Ips); err != nil {
		return err
	}

	// CNI will set the interface to up, and the kernel only sends GARPs/Unsolicited NA when the interface
	// goes from down to up or when the link-layer address changes on the interfaces. in order to the
	// kernel send GARPs/Unsolicited NA when the interface goes from down to up.
	// see https://github.com/spidernet-io/spiderpool/issues/4650
	var ipRes []net.IP
	for _, i := range ipamResponse.Payload.Ips {
		if i.Address != nil && *i.Address != "" {
			ipa, _, err := net.ParseCIDR(*i.Address)
			if err != nil {
				logger.Error(err.Error())
				continue
			}
			ipRes = append(ipRes, ipa)
		}
	}

	err = netns.Do(func(netNS ns.NetNS) error {
		return networking.AnnounceIPs(logger, args.IfName, ipRes)
	})

	if err != nil {
		logger.Error(err.Error())
	}

	// CNI will set the interface to up, and the kernel only sends GARPs/Unsolicited NA when the interface
	// goes from down to up or when the link-layer address changes on the interfaces. in order to the
	// kernel send GARPs/Unsolicited NA when the interface goes from down to up.
	// see https://github.com/spidernet-io/spiderpool/issues/4650
	var ipRes []net.IP
	for _, i := range ipamResponse.Payload.Ips {
		if i.Address != nil && *i.Address != "" {
			ipa, _, err := net.ParseCIDR(*i.Address)
			if err != nil {
				logger.Error(err.Error())
				continue
			}
			ipRes = append(ipRes, ipa)
		}
	}

	err = netns.Do(func(netNS ns.NetNS) error {
		return networking.AnnounceIPs(logger, args.IfName, ipRes)
	})

	if err != nil {
		logger.Error(err.Error())
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

	return result, nil
}
