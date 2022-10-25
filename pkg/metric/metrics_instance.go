// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

const (
	// spiderpool agent ipam allocation metrics name
	ipam_allocation_total_counts                 = "ipam_allocation_total_counts"
	ipam_allocation_failure_counts               = "ipam_allocation_failure_counts"
	ipam_allocation_rollback_failure_counts      = "ipam_allocation_rollback_failure_counts"
	ipam_allocation_err_internal_counts          = "ipam_allocation_err_internal_counts"
	ipam_allocation_err_no_available_pool_counts = "ipam_allocation_err_no_available_pool_counts"
	ipam_allocation_err_retries_exhausted_counts = "ipam_allocation_err_retries_exhausted_counts"
	ipam_allocation_err_ip_used_out_counts       = "ipam_allocation_err_ip_used_out_counts"

	ipam_allocation_average_duration_seconds = "ipam_allocation_average_duration_seconds"
	ipam_allocation_max_duration_seconds     = "ipam_allocation_max_duration_seconds"
	ipam_allocation_min_duration_seconds     = "ipam_allocation_min_duration_seconds"
	ipam_allocation_latest_duration_seconds  = "ipam_allocation_latest_duration_seconds"
	ipam_allocation_duration_seconds         = "ipam_allocation_duration_seconds"

	// spiderpool agent ipam release metrics name
	ipam_release_total_counts                 = "ipam_release_total_counts"
	ipam_release_failure_counts               = "ipam_release_failure_counts"
	ipam_release_err_internal_counts          = "ipam_release_err_internal_counts"
	ipam_release_err_retries_exhausted_counts = "ipam_release_err_retries_exhausted_counts"

	ipam_release_average_duration_seconds = "ipam_release_average_duration_seconds"
	ipam_release_max_duration_seconds     = "ipam_release_max_duration_seconds"
	ipam_release_min_duration_seconds     = "ipam_release_min_duration_seconds"
	ipam_release_latest_duration_seconds  = "ipam_release_latest_duration_seconds"
	ipam_release_duration_seconds         = "ipam_release_duration_seconds"

	// spiderpool controller IP GC metrics name
	ip_gc_total_counts   = "ip_gc_total_counts"
	ip_gc_failure_counts = "ip_gc_failure_counts"

	subnet_total_ip_counts = "subnet_total_ip_counts"
	subnet_free_ip_counts  = "subnet_free_ip_counts"

	ippool_total_ip_counts = "ippool_total_ip_counts"
	ippool_free_ip_counts  = "ippool_free_ip_counts"

	subnet_ippool_counts = "subnet_ippool_counts"
)

var (
	// spiderpool agent ipam allocation metrics
	IpamAllocationTotalCounts               syncint64.Counter
	IpamAllocationFailureCounts             syncint64.Counter
	IpamAllocationRollbackFailureCounts     syncint64.Counter
	IpamAllocationErrInternalCounts         syncint64.Counter
	IpamAllocationErrNoAvailablePoolCounts  syncint64.Counter
	IpamAllocationErrRetriesExhaustedCounts syncint64.Counter
	IpamAllocationErrIPUsedOutCounts        syncint64.Counter

	ipamAllocationAverageDurationSeconds syncfloat64.UpDownCounter
	ipamAllocationMaxDurationSeconds     syncfloat64.UpDownCounter
	ipamAllocationMinDurationSeconds     syncfloat64.UpDownCounter
	ipamAllocationLatestDurationSeconds  syncfloat64.UpDownCounter
	ipamAllocationDurationSeconds        syncfloat64.Histogram

	// spiderpool agent ipam release metrics
	IpamReleaseTotalCounts                 syncint64.Counter
	IpamReleaseFailureCounts               syncint64.Counter
	IpamReleasingErrInternalCounts         syncint64.Counter
	IpamReleasingErrRetriesExhaustedCounts syncint64.Counter

	ipamReleaseAverageDurationSeconds syncfloat64.UpDownCounter
	ipamReleaseMaxDurationSeconds     syncfloat64.UpDownCounter
	ipamReleaseMinDurationSeconds     syncfloat64.UpDownCounter
	ipamReleaseLatestDurationSeconds  syncfloat64.UpDownCounter
	ipamReleaseDurationSeconds        syncfloat64.Histogram

	// spiderpool controller IP GC metrics
	IPGCTotalCounts   syncint64.Counter
	IPGCFailureCounts syncint64.Counter

	SubnetTotalIPCounts syncint64.UpDownCounter
	SubnetFreeIPCounts  syncint64.UpDownCounter

	IPPoolTotalIPCounts syncint64.UpDownCounter
	IPPoolFreeIPCounts  syncint64.UpDownCounter

	SubnetIPPoolCounts syncint64.UpDownCounter
)

// InitSpiderpoolAgentMetrics serves for spiderpool agent metrics initialization
func InitSpiderpoolAgentMetrics(ctx context.Context) error {
	err := initSpiderpoolAgentAllocationMetrics(ctx)
	if nil != err {
		return err
	}

	err = initSpiderpoolAgentReleaseMetrics(ctx)
	if nil != err {
		return err
	}

	return nil
}

func initSpiderpoolAgentAllocationMetrics(ctx context.Context) error {
	// spiderpool agent ipam allocation total counts, metric type "int64 counter"
	allocationTotalCounts, err := NewMetricInt64Counter(ipam_allocation_total_counts, "spiderpool agent ipam allocation total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_total_counts, err)
	}
	IpamAllocationTotalCounts = allocationTotalCounts

	// spiderpool agent ipam allocation failure counts, metric type "int64 counter"
	allocationFailureCounts, err := NewMetricInt64Counter(ipam_allocation_failure_counts, "spiderpool agent ipam allocation failure counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_failure_counts, err)
	}
	IpamAllocationFailureCounts = allocationFailureCounts

	// spiderpool agent ipam allocation rollback failure counts, metric type "int64 counter"
	allocationRollbackFailureCounts, err := NewMetricInt64Counter(ipam_allocation_rollback_failure_counts, "spiderpool agent ipam allocation rollback failure counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_rollback_failure_counts, err)
	}
	IpamAllocationRollbackFailureCounts = allocationRollbackFailureCounts

	// spiderpool agent ipam allocation internal error counts, metric type "int64 counter"
	allocationErrInternalCounts, err := NewMetricInt64Counter(ipam_allocation_err_internal_counts, "spiderpool agent ipam allocation internal error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_err_internal_counts, err)
	}
	IpamAllocationErrInternalCounts = allocationErrInternalCounts

	// spiderpool agent ipam allocation no available IPPool error counts, metric type "int64 counter"
	allocationErrNoAvailablePoolCounts, err := NewMetricInt64Counter(ipam_allocation_err_no_available_pool_counts, "spiderpool agent ipam allocation no available IPPool error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_err_no_available_pool_counts, err)
	}
	IpamAllocationErrNoAvailablePoolCounts = allocationErrNoAvailablePoolCounts

	// spiderpool agent ipam allocation retries exhausted error counts, metric type "int64 counter"
	allocationErrRetriesExhaustedCounts, err := NewMetricInt64Counter(ipam_allocation_err_retries_exhausted_counts, "spiderpool agent ipam allocation retries exhausted error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_err_retries_exhausted_counts, err)
	}
	IpamAllocationErrRetriesExhaustedCounts = allocationErrRetriesExhaustedCounts

	// spiderpool agent ipam allocation IP addresses used out error counts, metric type "int64 counter"
	allocationErrIPUsedOutCounts, err := NewMetricInt64Counter(ipam_allocation_err_ip_used_out_counts, "spiderpool agent ipam allocation IP addresses used out error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_err_ip_used_out_counts, err)
	}
	IpamAllocationErrIPUsedOutCounts = allocationErrIPUsedOutCounts

	// spiderpool agent ipam average allocation duration, metric type "float64 gauge"
	allocationAvgDuration, err := NewMetricFloat64Gauge(ipam_allocation_average_duration_seconds, "spiderpool agent ipam average allocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_average_duration_seconds, err)
	}
	ipamAllocationAverageDurationSeconds = allocationAvgDuration

	// spiderpool agent ipam maximum allocation duration, metric type "float64 gauge"
	allocationMaxDuration, err := NewMetricFloat64Gauge(ipam_allocation_max_duration_seconds, "spiderpool agent ipam maximum allocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_max_duration_seconds, err)
	}
	ipamAllocationMaxDurationSeconds = allocationMaxDuration

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	allocationMinDuration, err := NewMetricFloat64Gauge(ipam_allocation_min_duration_seconds, "spiderpool agent ipam minimum allocation average duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_min_duration_seconds, err)
	}
	ipamAllocationMinDurationSeconds = allocationMinDuration

	// spiderpool agent ipam latest allocation duration, metric type "float64 gauge"
	allocationLatestDuration, err := NewMetricFloat64Gauge(ipam_allocation_latest_duration_seconds, "spiderpool agent ipam latest allocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_latest_duration_seconds, err)
	}
	ipamAllocationLatestDurationSeconds = allocationLatestDuration

	// spiderpool agent ipam allocation duration bucket, metric type "float64 histogram"
	allocationHistogram, err := NewMetricFloat64Histogram(ipam_allocation_duration_seconds, "spiderpool agent ipam allocation duration bucket")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_duration_seconds, err)
	}
	ipamAllocationDurationSeconds = allocationHistogram

	// set the spiderpool agent ipam allocation total counts initial data
	IpamAllocationTotalCounts.Add(ctx, 0)
	IpamAllocationFailureCounts.Add(ctx, 0)

	// set the spiderpool agent ipam average allocation duration initial data
	ipamAllocationAverageDurationSeconds.Add(ctx, 0)

	// set the spiderpool agent ipam maximum allocation duration initial data
	ipamAllocationMaxDurationSeconds.Add(ctx, 0)

	// set the spiderpool agent ipam minimum allocation duration initial data
	ipamAllocationMinDurationSeconds.Add(ctx, 0)

	// set the spiderpool agent ipam latest allocation duration initial data
	ipamAllocationLatestDurationSeconds.Add(ctx, 0)

	// set the spiderpool agent ipam allocation duration bucket initial data
	ipamAllocationDurationSeconds.Record(ctx, 0)

	return nil
}

func initSpiderpoolAgentReleaseMetrics(ctx context.Context) error {
	// spiderpool agent ipam release total counts, metric type "int64 counter"
	releaseTotalCounts, err := NewMetricInt64Counter(ipam_release_total_counts, "spiderpool agent ipam release total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_total_counts, err)
	}
	IpamReleaseTotalCounts = releaseTotalCounts

	// spiderpool agent ipam release failure counts, metric type "int64 counter"
	releaseFailureCounts, err := NewMetricInt64Counter(ipam_release_failure_counts, "spiderpool agent ipam release failure counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_failure_counts, err)
	}
	IpamReleaseFailureCounts = releaseFailureCounts

	// spiderpool agent ipam release internal error counts, metric type "int64 counter"
	releaseErrInternalCounts, err := NewMetricInt64Counter(ipam_release_err_internal_counts, "spiderpool agent ipam release internal error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_err_internal_counts, err)
	}
	IpamReleasingErrInternalCounts = releaseErrInternalCounts

	// spiderpool agent ipam release retries exhausted error counts, metric type "int64 counter"
	releaseErrRetriesExhaustedCounts, err := NewMetricInt64Counter(ipam_release_err_retries_exhausted_counts, "spiderpool agent ipam release retries exhausted error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_err_retries_exhausted_counts, err)
	}
	IpamReleasingErrRetriesExhaustedCounts = releaseErrRetriesExhaustedCounts

	// spiderpool agent ipam average release duration, metric type "float64 gauge"
	releaseAvgDuration, err := NewMetricFloat64Gauge(ipam_release_average_duration_seconds, "spiderpool agent ipam average release duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_average_duration_seconds, err)
	}
	ipamReleaseAverageDurationSeconds = releaseAvgDuration

	// spiderpool agent ipam maximum release duration, metric type "float64 gauge"
	releaseMaxDuration, err := NewMetricFloat64Gauge(ipam_release_max_duration_seconds, "spiderpool agent ipam maximum release duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_max_duration_seconds, err)
	}
	ipamReleaseMaxDurationSeconds = releaseMaxDuration

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	releaseMinDuration, err := NewMetricFloat64Gauge(ipam_release_min_duration_seconds, "spiderpool agent ipam minimum release average duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_min_duration_seconds, err)
	}
	ipamReleaseMinDurationSeconds = releaseMinDuration

	// spiderpool agent ipam latest release duration, metric type "float64 gauge"
	releaseLatestDuration, err := NewMetricFloat64Gauge(ipam_release_latest_duration_seconds, "spiderpool agent ipam latest release duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_latest_duration_seconds, err)
	}
	ipamReleaseLatestDurationSeconds = releaseLatestDuration

	// spiderpool agent ipam allocation duration bucket, metric type "float64 histogram"
	releaseHistogram, err := NewMetricFloat64Histogram(ipam_release_duration_seconds, "spiderpool agent ipam release duration bucket")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_duration_seconds, err)
	}
	ipamReleaseDurationSeconds = releaseHistogram

	// set the spiderpool agent ipam allocation total counts initial data
	IpamReleaseTotalCounts.Add(ctx, 0)
	IpamReleaseFailureCounts.Add(ctx, 0)

	// set the spiderpool agent ipam average allocation duration initial data
	ipamReleaseAverageDurationSeconds.Add(ctx, 0)

	// set the spiderpool agent ipam maximum allocation duration initial data
	ipamReleaseMaxDurationSeconds.Add(ctx, 0)

	// set the spiderpool agent ipam minimum allocation duration initial data
	ipamReleaseMinDurationSeconds.Add(ctx, 0)

	// set the spiderpool agent ipam latest allocation duration initial data
	ipamReleaseLatestDurationSeconds.Add(ctx, 0)

	// set the spiderpool agent ipam allocation duration bucket initial data
	ipamReleaseDurationSeconds.Record(ctx, 0)

	return nil
}

// InitSpiderpoolControllerMetrics serves for spiderpool controller metrics initialization
func InitSpiderpoolControllerMetrics(ctx context.Context) error {
	ipGCTotalCounts, err := NewMetricInt64Counter(ip_gc_total_counts, "spiderpool controller ip gc total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ip_gc_total_counts, err)
	}
	IPGCTotalCounts = ipGCTotalCounts

	ipGCFailureCounts, err := NewMetricInt64Counter(ip_gc_failure_counts, "spiderpool controller ip gc total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ip_gc_failure_counts, err)
	}
	IPGCFailureCounts = ipGCFailureCounts

	subnetTotalIPCounts, err := NewMetricInt64Gauge(subnet_total_ip_counts, "spider subnet total ip counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", subnet_total_ip_counts, err)
	}
	SubnetTotalIPCounts = subnetTotalIPCounts

	subnetFreeIPCounts, err := NewMetricInt64Gauge(subnet_free_ip_counts, "spider subnet free ip counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", subnet_free_ip_counts, err)
	}
	SubnetFreeIPCounts = subnetFreeIPCounts

	poolTotalIPCounts, err := NewMetricInt64Gauge(ippool_total_ip_counts, "spider ippool total ip counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ippool_total_ip_counts, err)
	}
	IPPoolTotalIPCounts = poolTotalIPCounts

	poolFreeIPCounts, err := NewMetricInt64Gauge(ippool_free_ip_counts, "spider ippool free ip counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ippool_free_ip_counts, err)
	}
	IPPoolFreeIPCounts = poolFreeIPCounts

	subnetIPPoolCounts, err := NewMetricInt64Gauge(subnet_ippool_counts, "spider subnet corresponding ippool counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", subnet_ippool_counts, err)
	}
	SubnetIPPoolCounts = subnetIPPoolCounts

	IPGCTotalCounts.Add(ctx, 0)
	IPGCFailureCounts.Add(ctx, 0)

	return nil
}
