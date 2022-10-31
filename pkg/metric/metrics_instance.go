// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"

	"github.com/spidernet-io/spiderpool/pkg/lock"
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

	ipamAllocationAverageDurationSeconds asyncFloat64Gauge
	ipamAllocationMaxDurationSeconds     asyncFloat64Gauge
	ipamAllocationMinDurationSeconds     asyncFloat64Gauge
	ipamAllocationLatestDurationSeconds  asyncFloat64Gauge
	ipamAllocationDurationSeconds        syncfloat64.Histogram

	// spiderpool agent ipam release metrics
	IpamReleaseTotalCounts               syncint64.Counter
	IpamReleaseFailureCounts             syncint64.Counter
	IpamReleaseErrInternalCounts         syncint64.Counter
	IpamReleaseErrRetriesExhaustedCounts syncint64.Counter

	ipamReleaseAverageDurationSeconds asyncFloat64Gauge
	ipamReleaseMaxDurationSeconds     asyncFloat64Gauge
	ipamReleaseMinDurationSeconds     asyncFloat64Gauge
	ipamReleaseLatestDurationSeconds  asyncFloat64Gauge
	ipamReleaseDurationSeconds        syncfloat64.Histogram

	// spiderpool controller IP GC metrics
	IPGCTotalCounts   syncint64.Counter
	IPGCFailureCounts syncint64.Counter

	SubnetPoolCounts asyncInt64Gauge
)

type gaugeCommon struct {
	observerLock          lock.RWMutex
	observerAttrsToReport *[]attribute.KeyValue
}

type asyncFloat64Gauge struct {
	gaugeMetric           asyncfloat64.Gauge
	observerValueToReport *float64
	gaugeCommon
}

func (ai *asyncFloat64Gauge) Record(value float64, attrs ...attribute.KeyValue) {
	ai.observerLock.Lock()
	*ai.observerValueToReport = value
	if len(attrs) != 0 {
		*ai.observerAttrsToReport = attrs
	}

	ai.observerLock.Unlock()
}

type asyncInt64Gauge struct {
	gaugeMetric           asyncint64.Gauge
	observerValueToReport *int64
	gaugeCommon
}

func (ai *asyncInt64Gauge) Record(value int64, attrs ...attribute.KeyValue) {
	ai.observerLock.Lock()
	*ai.observerValueToReport = value
	if len(attrs) != 0 {
		*ai.observerAttrsToReport = attrs
	}

	ai.observerLock.Unlock()
}

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
	ipamAllocationAverageDurationSeconds.gaugeMetric = allocationAvgDuration
	ipamAllocationAverageDurationSeconds.observerValueToReport = new(float64)
	ipamAllocationAverageDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamAllocationAverageDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamAllocationAverageDurationSeconds.observerLock.RLock()
		value := *ipamAllocationAverageDurationSeconds.observerValueToReport
		attrs := *ipamAllocationAverageDurationSeconds.observerAttrsToReport
		ipamAllocationAverageDurationSeconds.observerLock.RUnlock()
		ipamAllocationAverageDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_allocation_average_duration_seconds, err)
	}

	// spiderpool agent ipam maximum allocation duration, metric type "float64 gauge"
	allocationMaxDuration, err := NewMetricFloat64Gauge(ipam_allocation_max_duration_seconds, "spiderpool agent ipam maximum allocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_max_duration_seconds, err)
	}
	ipamAllocationMaxDurationSeconds.gaugeMetric = allocationMaxDuration
	ipamAllocationMaxDurationSeconds.observerValueToReport = new(float64)
	ipamAllocationMaxDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamAllocationMaxDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamAllocationMaxDurationSeconds.observerLock.RLock()
		value := *ipamAllocationMaxDurationSeconds.observerValueToReport
		attrs := *ipamAllocationMaxDurationSeconds.observerAttrsToReport
		ipamAllocationMaxDurationSeconds.observerLock.RUnlock()
		ipamAllocationMaxDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_allocation_max_duration_seconds, err)
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	allocationMinDuration, err := NewMetricFloat64Gauge(ipam_allocation_min_duration_seconds, "spiderpool agent ipam minimum allocation average duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_min_duration_seconds, err)
	}
	ipamAllocationMinDurationSeconds.gaugeMetric = allocationMinDuration
	ipamAllocationMinDurationSeconds.observerValueToReport = new(float64)
	ipamAllocationMinDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamAllocationMinDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamAllocationMinDurationSeconds.observerLock.RLock()
		value := *ipamAllocationMinDurationSeconds.observerValueToReport
		attrs := *ipamAllocationMinDurationSeconds.observerAttrsToReport
		ipamAllocationMinDurationSeconds.observerLock.RUnlock()
		ipamAllocationMinDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_allocation_min_duration_seconds, err)
	}

	// spiderpool agent ipam latest allocation duration, metric type "float64 gauge"
	allocationLatestDuration, err := NewMetricFloat64Gauge(ipam_allocation_latest_duration_seconds, "spiderpool agent ipam latest allocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_latest_duration_seconds, err)
	}
	ipamAllocationLatestDurationSeconds.gaugeMetric = allocationLatestDuration
	ipamAllocationLatestDurationSeconds.observerValueToReport = new(float64)
	ipamAllocationLatestDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamAllocationLatestDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamAllocationLatestDurationSeconds.observerLock.RLock()
		value := *ipamAllocationLatestDurationSeconds.observerValueToReport
		attrs := *ipamAllocationLatestDurationSeconds.observerAttrsToReport
		ipamAllocationLatestDurationSeconds.observerLock.RUnlock()
		ipamAllocationLatestDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_allocation_latest_duration_seconds, err)
	}

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
	ipamAllocationAverageDurationSeconds.observerLock.Lock()
	*ipamAllocationAverageDurationSeconds.observerValueToReport = 0
	ipamAllocationAverageDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam maximum allocation duration initial data
	ipamAllocationMaxDurationSeconds.observerLock.Lock()
	*ipamAllocationMaxDurationSeconds.observerValueToReport = 0
	ipamAllocationMaxDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam minimum allocation duration initial data
	ipamAllocationMinDurationSeconds.observerLock.Lock()
	*ipamAllocationMinDurationSeconds.observerValueToReport = 0
	ipamAllocationMinDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam latest allocation duration initial data
	ipamAllocationLatestDurationSeconds.observerLock.Lock()
	*ipamAllocationLatestDurationSeconds.observerValueToReport = 0
	ipamAllocationLatestDurationSeconds.observerLock.Unlock()

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

	// spiderpool agent ipam releasing internal error counts, metric type "int64 counter"
	releasingErrInternalCounts, err := NewMetricInt64Counter(ipam_release_err_internal_counts, "spiderpool agent ipam release internal error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_err_internal_counts, err)
	}
	IpamReleaseErrInternalCounts = releasingErrInternalCounts

	// spiderpool agent ipam releasing retries exhausted error counts, metric type "int64 counter"
	releasingErrRetriesExhaustedCounts, err := NewMetricInt64Counter(ipam_release_err_retries_exhausted_counts, "spiderpool agent ipam release retries exhausted error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_err_retries_exhausted_counts, err)
	}
	IpamReleaseErrRetriesExhaustedCounts = releasingErrRetriesExhaustedCounts

	// spiderpool agent ipam average release duration, metric type "float64 gauge"
	releaseAvgDuration, err := NewMetricFloat64Gauge(ipam_release_average_duration_seconds, "spiderpool agent ipam average release duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_average_duration_seconds, err)
	}
	ipamReleaseAverageDurationSeconds.gaugeMetric = releaseAvgDuration
	ipamReleaseAverageDurationSeconds.observerValueToReport = new(float64)
	ipamReleaseAverageDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamReleaseAverageDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamReleaseAverageDurationSeconds.observerLock.RLock()
		value := *ipamReleaseAverageDurationSeconds.observerValueToReport
		attrs := *ipamReleaseAverageDurationSeconds.observerAttrsToReport
		ipamReleaseAverageDurationSeconds.observerLock.RUnlock()
		ipamReleaseAverageDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_release_average_duration_seconds, err)
	}

	// spiderpool agent ipam maximum release duration, metric type "float64 gauge"
	releaseMaxDuration, err := NewMetricFloat64Gauge(ipam_release_max_duration_seconds, "spiderpool agent ipam maximum release duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_max_duration_seconds, err)
	}
	ipamReleaseMaxDurationSeconds.gaugeMetric = releaseMaxDuration
	ipamReleaseMaxDurationSeconds.observerValueToReport = new(float64)
	ipamReleaseMaxDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamReleaseMaxDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamReleaseMaxDurationSeconds.observerLock.RLock()
		value := *ipamReleaseMaxDurationSeconds.observerValueToReport
		attrs := *ipamReleaseMaxDurationSeconds.observerAttrsToReport
		ipamReleaseMaxDurationSeconds.observerLock.RUnlock()
		ipamReleaseMaxDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_release_max_duration_seconds, err)
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	releaseMinDuration, err := NewMetricFloat64Gauge(ipam_release_min_duration_seconds, "spiderpool agent ipam minimum release average duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_min_duration_seconds, err)
	}
	ipamReleaseMinDurationSeconds.gaugeMetric = releaseMinDuration
	ipamReleaseMinDurationSeconds.observerValueToReport = new(float64)
	ipamReleaseMinDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamReleaseMinDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamReleaseMinDurationSeconds.observerLock.RLock()
		value := *ipamReleaseMinDurationSeconds.observerValueToReport
		attrs := *ipamReleaseMinDurationSeconds.observerAttrsToReport
		ipamReleaseMinDurationSeconds.observerLock.RUnlock()
		ipamReleaseMinDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_release_min_duration_seconds, err)
	}

	// spiderpool agent ipam latest release duration, metric type "float64 gauge"
	releaseLatestDuration, err := NewMetricFloat64Gauge(ipam_release_latest_duration_seconds, "spiderpool agent ipam latest release duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_latest_duration_seconds, err)
	}
	ipamReleaseLatestDurationSeconds.gaugeMetric = releaseLatestDuration
	ipamReleaseLatestDurationSeconds.observerValueToReport = new(float64)
	ipamReleaseLatestDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamReleaseLatestDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamReleaseLatestDurationSeconds.observerLock.RLock()
		value := *ipamReleaseLatestDurationSeconds.observerValueToReport
		attrs := *ipamReleaseLatestDurationSeconds.observerAttrsToReport
		ipamReleaseLatestDurationSeconds.observerLock.RUnlock()
		ipamReleaseLatestDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_release_latest_duration_seconds, err)
	}

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
	ipamReleaseAverageDurationSeconds.observerLock.Lock()
	*ipamReleaseAverageDurationSeconds.observerValueToReport = 0
	ipamReleaseAverageDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam maximum allocation duration initial data
	ipamReleaseMaxDurationSeconds.observerLock.Lock()
	*ipamReleaseMaxDurationSeconds.observerValueToReport = 0
	ipamReleaseMaxDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam minimum allocation duration initial data
	ipamReleaseMinDurationSeconds.observerLock.Lock()
	*ipamReleaseMinDurationSeconds.observerValueToReport = 0
	ipamReleaseMinDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam latest allocation duration initial data
	ipamReleaseLatestDurationSeconds.observerLock.Lock()
	*ipamReleaseLatestDurationSeconds.observerValueToReport = 0
	ipamReleaseLatestDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam allocation duration bucket initial data
	ipamReleaseDurationSeconds.Record(ctx, 0)

	return nil
}

// InitSpiderpoolControllerMetrics serves for spiderpool controller metrics initialization
func InitSpiderpoolControllerMetrics(ctx context.Context) error {
	ipGCTotalCounts, err := NewMetricInt64Counter(ip_gc_total_counts, "spiderpool controller ip gc total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ip_gc_total_counts, err)
	}
	IPGCTotalCounts = ipGCTotalCounts

	ipGCFailureCounts, err := NewMetricInt64Counter(ip_gc_failure_counts, "spiderpool controller ip gc total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ip_gc_failure_counts, err)
	}
	IPGCFailureCounts = ipGCFailureCounts

	subnetPoolCounts, err := NewMetricInt64Gauge(subnet_ippool_counts, "spider subnet corresponding ippools counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", subnet_ippool_counts, err)
	}
	SubnetPoolCounts.gaugeMetric = subnetPoolCounts
	SubnetPoolCounts.observerValueToReport = new(int64)
	SubnetPoolCounts.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{SubnetPoolCounts.gaugeMetric}, func(ctx context.Context) {
		SubnetPoolCounts.observerLock.RLock()
		value := *SubnetPoolCounts.observerValueToReport
		attrs := *SubnetPoolCounts.observerAttrsToReport
		SubnetPoolCounts.observerLock.RUnlock()
		SubnetPoolCounts.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool controller metric '%s', error: %v", subnet_ippool_counts, err)
	}

	IPGCTotalCounts.Add(ctx, 0)
	IPGCFailureCounts.Add(ctx, 0)

	return nil
}
