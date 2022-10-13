// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"

	"github.com/go-openapi/runtime/middleware"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
)

// Singleton.
var (
	unixPostAgentIpamIp    = &_unixPostAgentIpamIp{}
	unixDeleteAgentIpamIp  = &_unixDeleteAgentIpamIp{}
	unixPostAgentIpamIps   = &_unixPostAgentIpamIps{}
	unixDeleteAgentIpamIps = &_unixDeleteAgentIpamIps{}
)

type _unixPostAgentIpamIp struct{}

// Handle handles POST requests for /ipam/ip.
func (g *_unixPostAgentIpamIp) Handle(params daemonset.PostIpamIPParams) middleware.Responder {
	logger := logutils.Logger.Named("IPAM").With(zap.String("CNICommand", "ADD"),
		zap.String("ContainerID", *params.IpamAddArgs.ContainerID),
		zap.String("IfName", *params.IpamAddArgs.IfName),
		zap.String("NetNamespace", *params.IpamAddArgs.NetNamespace),
		zap.String("PodNamespace", *params.IpamAddArgs.PodNamespace),
		zap.String("PodName", *params.IpamAddArgs.PodName),
	)
	ctx := logutils.IntoContext(params.HTTPRequest.Context(), logger)

	// The total count of IP allocations.
	metric.IpamAllocationTotalCounts.Add(ctx, 1)

	timeRecorder := metric.NewTimeRecorder()
	defer func() {
		// Time taken for once IP allocation.
		allocationDuration := timeRecorder.SinceInSeconds()
		metric.AllocDurationConstruct.RecordIPAMAllocationDuration(ctx, allocationDuration)
		logger.Sugar().Infof("IPAM allocation duration: %v", allocationDuration)
	}()

	resp, err := agentContext.IPAM.Allocate(ctx, params.IpamAddArgs)
	if err != nil {
		// The count of failures in IP allocations.
		metric.IpamAllocationFailureCounts.Add(ctx, 1)
		gatherIPAMAllocationErrMetric(ctx, err)
		logger.Error(err.Error())
		return daemonset.NewPostIpamIPFailure().WithPayload(models.Error(err.Error()))
	}

	return daemonset.NewPostIpamIPOK().WithPayload(resp)
}

type _unixDeleteAgentIpamIp struct{}

// Handle handles DELETE requests for /ipam/ip.
func (g *_unixDeleteAgentIpamIp) Handle(params daemonset.DeleteIpamIPParams) middleware.Responder {
	logger := logutils.Logger.Named("IPAM").With(zap.String("CNICommand", "DEL"),
		zap.String("ContainerID", *params.IpamDelArgs.ContainerID),
		zap.String("IfName", *params.IpamDelArgs.IfName),
		zap.String("NetNamespace", params.IpamDelArgs.NetNamespace),
		zap.String("PodNamespace", *params.IpamDelArgs.PodNamespace),
		zap.String("PodName", *params.IpamDelArgs.PodName),
	)
	ctx := logutils.IntoContext(params.HTTPRequest.Context(), logger)

	// The total count of IP releasing.
	metric.IpamDeallocationTotalCounts.Add(ctx, 1)

	timeRecorder := metric.NewTimeRecorder()
	defer func() {
		// Time taken for once IP releasing.
		deallocationDuration := timeRecorder.SinceInSeconds()
		metric.DeallocDurationConstruct.RecordIPAMDeallocationDuration(ctx, deallocationDuration)
		logger.Sugar().Infof("IPAM releasing duration: %v", deallocationDuration)
	}()

	if err := agentContext.IPAM.Release(ctx, params.IpamDelArgs); err != nil {
		// The count of failures in IP releasing.
		metric.IpamDeallocationFailureCounts.Add(ctx, 1)
		gatherIPAMReleasingErrMetric(ctx, err)
		logger.Error(err.Error())
		return daemonset.NewDeleteIpamIPFailure().WithPayload(models.Error(err.Error()))
	}

	return daemonset.NewDeleteIpamIPOK()
}

type _unixPostAgentIpamIps struct{}

// Handle handles POST requests for /ipam/ips.
func (g *_unixPostAgentIpamIps) Handle(params daemonset.PostIpamIpsParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return daemonset.NewPostIpamIpsOK()
}

type _unixDeleteAgentIpamIps struct{}

// Handle handles DELETE requests for /ipam/ips.
func (g *_unixDeleteAgentIpamIps) Handle(params daemonset.DeleteIpamIpsParams) middleware.Responder {
	// TODO (Icarus9913): return the http status code with logic.

	return daemonset.NewDeleteIpamIpsOK()
}

func gatherIPAMAllocationErrMetric(ctx context.Context, err error) {
	internal := true
	if errors.Is(err, constant.ErrWrongInput) {
		metric.IpamAllocationErrRetriesExhaustedCounts.Add(ctx, 1)
		internal = false
	}
	if errors.Is(err, constant.ErrNoAvailablePool) {
		metric.IpamAllocationErrNoAvailablePoolCounts.Add(ctx, 1)
		internal = false
	}
	if errors.Is(err, constant.ErrRetriesExhausted) {
		metric.IpamAllocationErrRetriesExhaustedCounts.Add(ctx, 1)
		internal = false
	}
	if errors.Is(err, constant.ErrIPUsedOut) {
		metric.IpamAllocationErrIPUsedOutCounts.Add(ctx, 1)
		internal = false
	}

	if internal {
		metric.IpamAllocationErrInternalCounts.Add(ctx, 1)
	}
}

func gatherIPAMReleasingErrMetric(ctx context.Context, err error) {
	internal := true
	if errors.Is(err, constant.ErrRetriesExhausted) {
		metric.IpamReleasingErrRetriesExhaustedCounts.Add(ctx, 1)
		internal = false
	}

	if internal {
		metric.IpamReleasingErrInternalCounts.Add(ctx, 1)
	}
}
