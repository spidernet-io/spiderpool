// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"

	"github.com/go-openapi/runtime/middleware"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	metrics "github.com/spidernet-io/spiderpool/pkg/metric"
)

// Singleton
var (
	unixPostAgentIpamIp    = &_unixPostAgentIpamIp{}
	unixDeleteAgentIpamIp  = &_unixDeleteAgentIpamIp{}
	unixPostAgentIpamIps   = &_unixPostAgentIpamIps{}
	unixDeleteAgentIpamIps = &_unixDeleteAgentIpamIps{}
)

type _unixPostAgentIpamIp struct{}

// Handle handles POST requests for /ipam/ip .
func (g *_unixPostAgentIpamIp) Handle(params daemonset.PostIpamIPParams) middleware.Responder {
	logger := logutils.Logger.Named("IPAM").With(zap.String("CNICommand", "ADD"),
		zap.String("ContainerID", *params.IpamAddArgs.ContainerID),
		zap.String("IfName", *params.IpamAddArgs.IfName),
		zap.String("NetNamespace", *params.IpamAddArgs.NetNamespace),
		zap.String("PodNamespace", *params.IpamAddArgs.PodNamespace),
		zap.String("PodName", *params.IpamAddArgs.PodName),
	)
	ctx := logutils.IntoContext(params.HTTPRequest.Context(), logger)

	// spiderpool agent ipam allocation total counts metric
	metrics.IpamAllocationTotalCounts.Add(ctx, 1)

	timeRecorder := metrics.NewTimeRecorder()
	defer func() {
		// record spiderpool agent allocation duration metrics
		allocationDuration := timeRecorder.SinceInSeconds()
		metrics.AllocDurationConstruct.RecordIPAMAllocationDuration(ctx, allocationDuration)
		logger.Sugar().Infof("IPAM allocation duration: %v", allocationDuration)
	}()

	resp, err := agentContext.IPAM.Allocate(ctx, params.IpamAddArgs)
	if err != nil {
		// spiderpool agent ipam allocation failed counts metric
		metrics.IpamAllocationFailureCounts.Add(ctx, 1)

		logger.Sugar().Errorf("Failed to allocate: %v", err)
		if errors.Is(err, constant.ErrWrongInput) {
			return daemonset.NewPostIpamIPStatus512()
		}
		if errors.Is(err, constant.ErrNotAllocatablePod) {
			return daemonset.NewPostIpamIPStatus513()
		}
		if errors.Is(err, constant.ErrNoAvailablePool) {
			return daemonset.NewPostIpamIPStatus514()
		}
		if errors.Is(err, constant.ErrIPUsedOut) {
			return daemonset.NewPostIpamIPStatus515()
		}
		return daemonset.NewPostIpamIPInternalServerError()
	}

	return daemonset.NewPostIpamIPOK().WithPayload(resp)
}

type _unixDeleteAgentIpamIp struct{}

// Handle handles DELETE requests for /ipam/ip .
func (g *_unixDeleteAgentIpamIp) Handle(params daemonset.DeleteIpamIPParams) middleware.Responder {
	logger := logutils.Logger.Named("IPAM").With(zap.String("CNICommand", "DEL"),
		zap.String("ContainerID", *params.IpamDelArgs.ContainerID),
		zap.String("IfName", *params.IpamDelArgs.IfName),
		zap.String("NetNamespace", params.IpamDelArgs.NetNamespace),
		zap.String("PodNamespace", *params.IpamDelArgs.PodNamespace),
		zap.String("PodName", *params.IpamDelArgs.PodName),
	)
	ctx := logutils.IntoContext(params.HTTPRequest.Context(), logger)

	// spiderpool agent ipam deallocation total counts metric
	metrics.IpamDeallocationTotalCounts.Add(ctx, 1)

	timeRecorder := metrics.NewTimeRecorder()
	defer func() {
		// record spiderpool agent deallocation duration metrics
		deallocationDuration := timeRecorder.SinceInSeconds()
		metrics.DeallocDurationConstruct.RecordIPAMDeallocationDuration(ctx, deallocationDuration)
		logger.Sugar().Infof("IPAM deallocation duration: %v", deallocationDuration)
	}()

	if err := agentContext.IPAM.Release(ctx, params.IpamDelArgs); err != nil {
		metrics.IpamDeallocationFailureCounts.Add(ctx, 1)

		logger.Sugar().Errorf("Failed to release: %v", err)
		return daemonset.NewDeleteIpamIPInternalServerError()
	}

	return daemonset.NewDeleteIpamIPOK()
}

type _unixPostAgentIpamIps struct{}

// Handle handles POST requests for /ipam/ips .
func (g *_unixPostAgentIpamIps) Handle(params daemonset.PostIpamIpsParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return daemonset.NewPostIpamIpsOK()
}

type _unixDeleteAgentIpamIps struct{}

// Handle handles DELETE requests for /ipam/ips .
func (g *_unixDeleteAgentIpamIps) Handle(params daemonset.DeleteIpamIpsParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return daemonset.NewDeleteIpamIpsOK()
}
