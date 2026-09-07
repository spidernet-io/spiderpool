// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
)

// IaaS prewarm/global pool metrics names. These cover the Spiderpool
// (consumer) side only; producer-side metrics such as prewarm watermarks,
// cloud API calls and reconcile latency belong to the external IaaS
// provider.
const (
	iaasAllocationCountsName            = metricPrefix + "iaas_allocation_total"
	iaasAllocationFailureCountsName     = metricPrefix + "iaas_allocation_failure_total"
	iaasAllocationDurationSecondsName   = metricPrefix + "iaas_allocation_duration_seconds"
	iaasRPCDurationSecondsName          = metricPrefix + "iaas_rpc_duration_seconds"
	iaasRPCFailureCountsName            = metricPrefix + "iaas_rpc_failure_total"
	iaasClaimRollbackCountsName         = metricPrefix + "iaas_claim_rollback_total"
	iaasMetadataDecodeFailureCountsName = metricPrefix + "iaas_metadata_decode_failure_total"
)

// Attribute values for the IaaS metrics labels.
const (
	// "mode" label: node-level prewarm pool vs global pool.
	IaaSModeNode   = "node"
	IaaSModeGlobal = "global"

	// The "path" label values are the types.IaaSAllocationPath enum:
	// cache_hit (zero-RPC reuse of prewarmed metadata), cold_create
	// (provider creates+attaches a new sub-ENI), cold_rebind (provider
	// re-attaches a detached sub-ENI) and cold_steal (provider
	// detaches an idle sub-ENI from another node and attaches it locally).

	// "op" label of the provider RPC metrics.
	IaaSOpAllocate = "allocate"
	IaaSOpRelease  = "release"

	// "reason" label of the allocation failure counter.
	IaaSAllocFailReasonNoReadyIP        = "no_ready_ip"
	IaaSAllocFailReasonPairConflict     = "pair_conflict"
	IaaSAllocFailReasonMetadataNotReady = "metadata_not_ready"
	IaaSAllocFailReasonRPCError         = "rpc_error"
	IaaSAllocFailReasonInternal         = "internal"

	// "reason" label of the RPC failure counter (client-side view).
	IaaSRPCFailReasonTimeout      = "timeout"
	IaaSRPCFailReasonNetworkError = "network_error"
	IaaSRPCFailReasonHTTPStatus   = "http_status"
	IaaSRPCFailReasonBadResponse  = "bad_response"

	// "reason" label of the metadata decode failure counter.
	IaaSMetadataFailReasonBadJSON       = "bad_json"
	IaaSMetadataFailReasonMissingScope  = "missing_scope"
	IaaSMetadataFailReasonScopeMismatch = "scope_mismatch"
)

const (
	iaasAttrKeyMode   = attribute.Key("mode")
	iaasAttrKeyPath   = attribute.Key("path")
	iaasAttrKeyPool   = attribute.Key("pool")
	iaasAttrKeyOp     = attribute.Key("op")
	iaasAttrKeyReason = attribute.Key("reason")
)

var (
	iaasAllocationTotalCounts              api.Int64Counter
	iaasAllocationFailureCounts            api.Int64Counter
	iaasAllocationDurationSecondsHistogram api.Float64Histogram
	iaasRPCDurationSecondsHistogram        api.Float64Histogram
	iaasRPCFailureCounts                   api.Int64Counter
	iaasClaimRollbackCounts                api.Int64Counter
	iaasMetadataDecodeFailureCounts        api.Int64Counter
)

// initSpiderpoolAgentIaaSMetrics will init spiderpool-agent IaaS pool metrics.
func initSpiderpoolAgentIaaSMetrics(ctx context.Context) error {
	allocationTotalCounts, err := newMetricInt64Counter(iaasAllocationCountsName, "spiderpool agent IaaS pool allocation total counts by mode, path and pool", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", iaasAllocationCountsName, err)
	}
	iaasAllocationTotalCounts = allocationTotalCounts

	allocationFailureCounts, err := newMetricInt64Counter(iaasAllocationFailureCountsName, "spiderpool agent IaaS pool allocation failure counts by mode, pool and reason", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", iaasAllocationFailureCountsName, err)
	}
	iaasAllocationFailureCounts = allocationFailureCounts

	allocationDurationSecondsHistogram, err := newMetricFloat64Histogram(iaasAllocationDurationSecondsName, "spiderpool agent end-to-end IaaS pool IP allocation duration in seconds by mode, path and pool, including the cold-path provider Allocate RPC", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", iaasAllocationDurationSecondsName, err)
	}
	iaasAllocationDurationSecondsHistogram = allocationDurationSecondsHistogram

	rpcDurationSecondsHistogram, err := newMetricFloat64Histogram(iaasRPCDurationSecondsName, "spiderpool agent IaaS provider RPC duration in seconds (client-side view)", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", iaasRPCDurationSecondsName, err)
	}
	iaasRPCDurationSecondsHistogram = rpcDurationSecondsHistogram

	rpcFailureCounts, err := newMetricInt64Counter(iaasRPCFailureCountsName, "spiderpool agent IaaS provider RPC failure counts by op and reason (client-side view)", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", iaasRPCFailureCountsName, err)
	}
	iaasRPCFailureCounts = rpcFailureCounts

	claimRollbackCounts, err := newMetricInt64Counter(iaasClaimRollbackCountsName, "spiderpool agent IaaS global pool cold-path claim rollback counts after a failed provider allocate RPC", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", iaasClaimRollbackCountsName, err)
	}
	iaasClaimRollbackCounts = claimRollbackCounts

	metadataDecodeFailureCounts, err := newMetricInt64Counter(iaasMetadataDecodeFailureCountsName, "spiderpool agent IaaS pool metadata decode failure counts by reason", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", iaasMetadataDecodeFailureCountsName, err)
	}
	iaasMetadataDecodeFailureCounts = metadataDecodeFailureCounts

	return nil
}

// RecordIaaSAllocation increases the IaaS allocation total counter. The
// recorder is nil-safe so that shared packages (ippoolmanager, iaas client)
// can call it from processes that never initialize the agent metrics.
func RecordIaaSAllocation(ctx context.Context, mode, path, pool string) {
	if !globalEnableMetric || iaasAllocationTotalCounts == nil {
		return
	}
	iaasAllocationTotalCounts.Add(ctx, 1, api.WithAttributes(
		iaasAttrKeyMode.String(mode),
		iaasAttrKeyPath.String(path),
		iaasAttrKeyPool.String(pool),
	))
}

// RecordIaaSAllocationFailure increases the IaaS allocation failure counter.
func RecordIaaSAllocationFailure(ctx context.Context, mode, reason, pool string) {
	if !globalEnableMetric || iaasAllocationFailureCounts == nil {
		return
	}
	iaasAllocationFailureCounts.Add(ctx, 1, api.WithAttributes(
		iaasAttrKeyMode.String(mode),
		iaasAttrKeyReason.String(reason),
		iaasAttrKeyPool.String(pool),
	))
}

// RecordIaaSAllocationDuration records one end-to-end IaaS pool IP
// allocation duration (pool claim plus, on cold paths, the synchronous
// provider Allocate RPC), labeled by mode, selection path and pool name.
func RecordIaaSAllocationDuration(ctx context.Context, mode, path, pool string, durationSeconds float64) {
	if !globalEnableMetric || iaasAllocationDurationSecondsHistogram == nil {
		return
	}
	iaasAllocationDurationSecondsHistogram.Record(ctx, durationSeconds, api.WithAttributes(
		iaasAttrKeyMode.String(mode),
		iaasAttrKeyPath.String(path),
		iaasAttrKeyPool.String(pool),
	))
}

// RecordIaaSRPCDuration records one client-side IaaS provider RPC duration.
func RecordIaaSRPCDuration(ctx context.Context, op string, durationSeconds float64) {
	if !globalEnableMetric || iaasRPCDurationSecondsHistogram == nil {
		return
	}
	iaasRPCDurationSecondsHistogram.Record(ctx, durationSeconds, api.WithAttributes(
		iaasAttrKeyOp.String(op),
	))
}

// RecordIaaSRPCFailure increases the client-side IaaS provider RPC failure counter.
func RecordIaaSRPCFailure(ctx context.Context, op, reason string) {
	if !globalEnableMetric || iaasRPCFailureCounts == nil {
		return
	}
	iaasRPCFailureCounts.Add(ctx, 1, api.WithAttributes(
		iaasAttrKeyOp.String(op),
		iaasAttrKeyReason.String(reason),
	))
}

// RecordIaaSClaimRollback increases the global pool cold-path claim rollback counter.
func RecordIaaSClaimRollback(ctx context.Context) {
	if !globalEnableMetric || iaasClaimRollbackCounts == nil {
		return
	}
	iaasClaimRollbackCounts.Add(ctx, 1)
}

// RecordIaaSMetadataDecodeFailure increases the IaaS pool metadata decode failure counter.
func RecordIaaSMetadataDecodeFailure(ctx context.Context, reason string) {
	if !globalEnableMetric || iaasMetadataDecodeFailureCounts == nil {
		return
	}
	iaasMetadataDecodeFailureCounts.Add(ctx, 1, api.WithAttributes(
		iaasAttrKeyReason.String(reason),
	))
}
