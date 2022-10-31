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

	// spiderpool agent ipam deallocation metrics name
	ipam_deallocation_total_counts              = "ipam_deallocation_total_counts"
	ipam_deallocation_failure_counts            = "ipam_deallocation_failure_counts"
	ipam_releasing_err_internal_counts          = "ipam_releasing_err_internal_counts"
	ipam_releasing_err_retries_exhausted_counts = "ipam_releasing_err_retries_exhausted_counts"

	ipam_deallocation_average_duration_seconds = "ipam_deallocation_average_duration_seconds"
	ipam_deallocation_max_duration_seconds     = "ipam_deallocation_max_duration_seconds"
	ipam_deallocation_min_duration_seconds     = "ipam_deallocation_min_duration_seconds"
	ipam_deallocation_latest_duration_seconds  = "ipam_deallocation_latest_duration_seconds"
	ipam_deallocation_duration_seconds         = "ipam_deallocation_duration_seconds"

	// spiderpool controller IP GC metrics name
	ip_gc_total_counts   = "ip_gc_total_counts"
	ip_gc_failure_counts = "ip_gc_failure_counts"

	subne_ippool_counts = "subne_ippool_counts"
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

	// spiderpool agent ipam deallocation metrics
	IpamDeallocationTotalCounts            syncint64.Counter
	IpamDeallocationFailureCounts          syncint64.Counter
	IpamReleasingErrInternalCounts         syncint64.Counter
	IpamReleasingErrRetriesExhaustedCounts syncint64.Counter

	ipamDeallocationAverageDurationSeconds asyncFloat64Gauge
	ipamDeallocationMaxDurationSeconds     asyncFloat64Gauge
	ipamDeallocationMinDurationSeconds     asyncFloat64Gauge
	ipamDeallocationLatestDurationSeconds  asyncFloat64Gauge
	ipamDeallocationDurationSeconds        syncfloat64.Histogram

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

	err = initSpiderpoolAgentDeallocationMetrics(ctx)
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

func initSpiderpoolAgentDeallocationMetrics(ctx context.Context) error {
	// spiderpool agent ipam deallocation total counts, metric type "int64 counter"
	deallocationTotalCounts, err := NewMetricInt64Counter(ipam_deallocation_total_counts, "spiderpool agent ipam deallocation total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_deallocation_total_counts, err)
	}
	IpamDeallocationTotalCounts = deallocationTotalCounts

	// spiderpool agent ipam deallocation failure counts, metric type "int64 counter"
	deallocationFailureCounts, err := NewMetricInt64Counter(ipam_deallocation_failure_counts, "spiderpool agent ipam deallocation failure counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_deallocation_failure_counts, err)
	}
	IpamDeallocationFailureCounts = deallocationFailureCounts

	// spiderpool agent ipam releasing internal error counts, metric type "int64 counter"
	releasingErrInternalCounts, err := NewMetricInt64Counter(ipam_releasing_err_internal_counts, "spiderpool agent ipam releasing internal error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_releasing_err_internal_counts, err)
	}
	IpamReleasingErrInternalCounts = releasingErrInternalCounts

	// spiderpool agent ipam releasing retries exhausted error counts, metric type "int64 counter"
	releasingErrRetriesExhaustedCounts, err := NewMetricInt64Counter(ipam_releasing_err_retries_exhausted_counts, "spiderpool agent ipam releasing retries exhausted error counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_releasing_err_retries_exhausted_counts, err)
	}
	IpamReleasingErrRetriesExhaustedCounts = releasingErrRetriesExhaustedCounts

	// spiderpool agent ipam average deallocation duration, metric type "float64 gauge"
	deallocationAvgDuration, err := NewMetricFloat64Gauge(ipam_deallocation_average_duration_seconds, "spiderpool agent ipam average deallocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_deallocation_average_duration_seconds, err)
	}
	ipamDeallocationAverageDurationSeconds.gaugeMetric = deallocationAvgDuration
	ipamDeallocationAverageDurationSeconds.observerValueToReport = new(float64)
	ipamDeallocationAverageDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamDeallocationAverageDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamDeallocationAverageDurationSeconds.observerLock.RLock()
		value := *ipamDeallocationAverageDurationSeconds.observerValueToReport
		attrs := *ipamDeallocationAverageDurationSeconds.observerAttrsToReport
		ipamDeallocationAverageDurationSeconds.observerLock.RUnlock()
		ipamDeallocationAverageDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_deallocation_average_duration_seconds, err)
	}

	// spiderpool agent ipam maximum deallocation duration, metric type "float64 gauge"
	deallocationMaxDuration, err := NewMetricFloat64Gauge(ipam_deallocation_max_duration_seconds, "spiderpool agent ipam maximum deallocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_deallocation_max_duration_seconds, err)
	}
	ipamDeallocationMaxDurationSeconds.gaugeMetric = deallocationMaxDuration
	ipamDeallocationMaxDurationSeconds.observerValueToReport = new(float64)
	ipamDeallocationMaxDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamDeallocationMaxDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamDeallocationMaxDurationSeconds.observerLock.RLock()
		value := *ipamDeallocationMaxDurationSeconds.observerValueToReport
		attrs := *ipamDeallocationMaxDurationSeconds.observerAttrsToReport
		ipamDeallocationMaxDurationSeconds.observerLock.RUnlock()
		ipamDeallocationMaxDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_deallocation_max_duration_seconds, err)
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	deallocationMinDuration, err := NewMetricFloat64Gauge(ipam_deallocation_min_duration_seconds, "spiderpool agent ipam minimum deallocation average duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_deallocation_min_duration_seconds, err)
	}
	ipamDeallocationMinDurationSeconds.gaugeMetric = deallocationMinDuration
	ipamDeallocationMinDurationSeconds.observerValueToReport = new(float64)
	ipamDeallocationMinDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamDeallocationMinDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamDeallocationMinDurationSeconds.observerLock.RLock()
		value := *ipamDeallocationMinDurationSeconds.observerValueToReport
		attrs := *ipamDeallocationMinDurationSeconds.observerAttrsToReport
		ipamDeallocationMinDurationSeconds.observerLock.RUnlock()
		ipamDeallocationMinDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_deallocation_min_duration_seconds, err)
	}

	// spiderpool agent ipam latest deallocation duration, metric type "float64 gauge"
	deallocationLatestDuration, err := NewMetricFloat64Gauge(ipam_deallocation_latest_duration_seconds, "spiderpool agent ipam latest deallocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_deallocation_latest_duration_seconds, err)
	}
	ipamDeallocationLatestDurationSeconds.gaugeMetric = deallocationLatestDuration
	ipamDeallocationLatestDurationSeconds.observerValueToReport = new(float64)
	ipamDeallocationLatestDurationSeconds.observerAttrsToReport = new([]attribute.KeyValue)
	err = meter.RegisterCallback([]instrument.Asynchronous{ipamDeallocationLatestDurationSeconds.gaugeMetric}, func(ctx context.Context) {
		ipamDeallocationLatestDurationSeconds.observerLock.RLock()
		value := *ipamDeallocationLatestDurationSeconds.observerValueToReport
		attrs := *ipamDeallocationLatestDurationSeconds.observerAttrsToReport
		ipamDeallocationLatestDurationSeconds.observerLock.RUnlock()
		ipamDeallocationLatestDurationSeconds.gaugeMetric.Observe(ctx, value, attrs...)
	})
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool agent metric '%s', error: %v", ipam_deallocation_latest_duration_seconds, err)
	}

	// spiderpool agent ipam allocation duration bucket, metric type "float64 histogram"
	deallocationHistogram, err := NewMetricFloat64Histogram(ipam_deallocation_duration_seconds, "spiderpool agent ipam deallocation duration bucket")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_deallocation_duration_seconds, err)
	}
	ipamDeallocationDurationSeconds = deallocationHistogram

	// set the spiderpool agent ipam allocation total counts initial data
	IpamDeallocationTotalCounts.Add(ctx, 0)
	IpamDeallocationFailureCounts.Add(ctx, 0)

	// set the spiderpool agent ipam average allocation duration initial data
	ipamDeallocationAverageDurationSeconds.observerLock.Lock()
	*ipamDeallocationAverageDurationSeconds.observerValueToReport = 0
	ipamDeallocationAverageDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam maximum allocation duration initial data
	ipamDeallocationMaxDurationSeconds.observerLock.Lock()
	*ipamDeallocationMaxDurationSeconds.observerValueToReport = 0
	ipamDeallocationMaxDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam minimum allocation duration initial data
	ipamDeallocationMinDurationSeconds.observerLock.Lock()
	*ipamDeallocationMinDurationSeconds.observerValueToReport = 0
	ipamDeallocationMinDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam latest allocation duration initial data
	ipamDeallocationLatestDurationSeconds.observerLock.Lock()
	*ipamDeallocationLatestDurationSeconds.observerValueToReport = 0
	ipamDeallocationLatestDurationSeconds.observerLock.Unlock()

	// set the spiderpool agent ipam allocation duration bucket initial data
	ipamDeallocationDurationSeconds.Record(ctx, 0)

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

	subnetPoolCounts, err := NewMetricInt64Gauge(subne_ippool_counts, "spider subnet corresponding ippools counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", subne_ippool_counts, err)
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
		return fmt.Errorf("failed to register callback for spiderpool controller metric '%s', error: %v", subne_ippool_counts, err)
	}

	IPGCTotalCounts.Add(ctx, 0)
	IPGCFailureCounts.Add(ctx, 0)

	return nil
}
