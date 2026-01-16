// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"errors"

	"github.com/go-openapi/runtime/middleware"
	"github.com/go-openapi/strfmt"
	"go.uber.org/zap"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/api/v1/agent/server/restapi/daemonset"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/metric"
)

// Singleton.
var (
	unixPostAgentIpamIP    = &_unixPostAgentIpamIP{}
	unixDeleteAgentIpamIP  = &_unixDeleteAgentIpamIP{}
	unixPostAgentIpamIps   = &_unixPostAgentIpamIps{}
	unixDeleteAgentIpamIps = &_unixDeleteAgentIpamIps{}
)

type _unixPostAgentIpamIP struct{}

// Handle handles POST requests for /ipam/ip.
func (g *_unixPostAgentIpamIP) Handle(params daemonset.PostIpamIPParams) middleware.Responder {
	if err := params.IpamAddArgs.Validate(strfmt.Default); err != nil {
		return daemonset.NewPostIpamIPFailure().WithPayload(models.Error(err.Error()))
	}

	logger := logutils.Logger.Named("IPAM").With(
		zap.String("CNICommand", "ADD"),
		zap.String("ContainerID", *params.IpamAddArgs.ContainerID),
		zap.String("IfName", *params.IpamAddArgs.IfName),
		zap.String("NetNamespace", *params.IpamAddArgs.NetNamespace),
		zap.String("PodNamespace", *params.IpamAddArgs.PodNamespace),
		zap.String("PodName", *params.IpamAddArgs.PodName),
		zap.String("PodUID", *params.IpamAddArgs.PodUID),
	)
	ctx := logutils.IntoContext(params.HTTPRequest.Context(), logger)

	// The total count of IP allocations.
	metric.IpamAllocationTotalCounts.Add(ctx, 1)

	timeRecorder := metric.NewTimeRecorder()
	defer func() {
		// Time taken for once IP allocation.
		allocationDuration := timeRecorder.SinceInSeconds()
		metric.IPAMDurationConstruct.RecordIPAMAllocationDuration(ctx, allocationDuration)
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

type _unixDeleteAgentIpamIP struct{}

// Handle handles DELETE requests for /ipam/ip.
func (g *_unixDeleteAgentIpamIP) Handle(params daemonset.DeleteIpamIPParams) middleware.Responder {
	if err := params.IpamDelArgs.Validate(strfmt.Default); err != nil {
		return daemonset.NewDeleteIpamIPFailure().WithPayload(models.Error(err.Error()))
	}

	logger := logutils.Logger.Named("IPAM").With(
		zap.String("CNICommand", "DEL"),
		zap.String("ContainerID", *params.IpamDelArgs.ContainerID),
		zap.String("IfName", *params.IpamDelArgs.IfName),
		zap.String("NetNamespace", params.IpamDelArgs.NetNamespace),
		zap.String("PodNamespace", *params.IpamDelArgs.PodNamespace),
		zap.String("PodName", *params.IpamDelArgs.PodName),
		zap.String("PodUID", *params.IpamDelArgs.PodUID),
	)
	ctx := logutils.IntoContext(params.HTTPRequest.Context(), logger)

	// The total count of IP releasing.
	metric.IpamReleaseTotalCounts.Add(ctx, 1)

	timeRecorder := metric.NewTimeRecorder()
	defer func() {
		// Time taken for once IP releasing.
		releaseDuration := timeRecorder.SinceInSeconds()
		metric.IPAMDurationConstruct.RecordIPAMReleaseDuration(ctx, releaseDuration)
		logger.Sugar().Infof("IPAM releasing duration: %v", releaseDuration)
	}()

	if err := agentContext.IPAM.Release(ctx, params.IpamDelArgs); err != nil {
		// The count of failures in IP releasing.
		metric.IpamReleaseFailureCounts.Add(ctx, 1)
		gatherIPAMReleasingErrMetric(ctx, err)
		logger.Error(err.Error())

		return daemonset.NewDeleteIpamIPFailure().WithPayload(models.Error(err.Error()))
	}

	return daemonset.NewDeleteIpamIPOK()
}

type _unixPostAgentIpamIps struct{}

// Handle handles POST requests for /ipam/ips.
func (g *_unixPostAgentIpamIps) Handle(params daemonset.PostIpamIpsParams) middleware.Responder {
	return daemonset.NewPostIpamIpsOK()
}

type _unixDeleteAgentIpamIps struct{}

// Handle handles DELETE requests for /ipam/ips.
func (g *_unixDeleteAgentIpamIps) Handle(params daemonset.DeleteIpamIpsParams) middleware.Responder {
	err := params.IpamBatchDelArgs.Validate(strfmt.Default)
	if err != nil {
		return daemonset.NewDeleteIpamIpsFailure().WithPayload(models.Error(err.Error()))
	}

	log := logutils.Logger.Named("IPAM").With(
		zap.String("Operation", "Release IPs"),
		zap.String("ContainerID", *params.IpamBatchDelArgs.ContainerID),
		zap.String("NetNamespace", params.IpamBatchDelArgs.NetNamespace),
		zap.String("PodNamespace", *params.IpamBatchDelArgs.PodNamespace),
		zap.String("PodName", *params.IpamBatchDelArgs.PodName),
		zap.String("PodUID", *params.IpamBatchDelArgs.PodUID),
	)
	ctx := logutils.IntoContext(params.HTTPRequest.Context(), log)

	// The total count of IP releasing.
	metric.IpamReleaseTotalCounts.Add(ctx, 1)

	timeRecorder := metric.NewTimeRecorder()
	defer func() {
		// Time taken for once IP releasing.
		releaseDuration := timeRecorder.SinceInSeconds()
		metric.IPAMDurationConstruct.RecordIPAMReleaseDuration(ctx, releaseDuration)
		logger.Sugar().Infof("IPAM releasing duration: %v", releaseDuration)
	}()

	err = agentContext.IPAM.ReleaseIPs(ctx, params.IpamBatchDelArgs)
	if nil != err {
		// The count of failures in IP releasing.
		metric.IpamReleaseFailureCounts.Add(ctx, 1)
		gatherIPAMReleasingErrMetric(ctx, err)
		logger.Error(err.Error())
		return filteredErrResponder(err)
	}

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
		metric.IpamReleaseErrRetriesExhaustedCounts.Add(ctx, 1)
		internal = false
	}

	if internal {
		metric.IpamReleaseErrInternalCounts.Add(ctx, 1)
	}
}

func filteredErrResponder(err error) middleware.Responder {
	switch {
	case errors.Is(err, constant.ErrForbidReleasingStatelessWorkload):
		return daemonset.NewDeleteIpamIpsStatus521().WithPayload(models.Error(err.Error()))
	case errors.Is(err, constant.ErrForbidReleasingStatefulWorkload):
		return daemonset.NewDeleteIpamIpsStatus522().WithPayload(models.Error(err.Error()))
	default:
		return daemonset.NewDeleteIpamIpsFailure().WithPayload(models.Error(err.Error()))
	}
}
