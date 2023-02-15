// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/instrument"

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
	ipam_allocation_duration_seconds         = "ipam_allocation_duration_seconds_histogram"

	// spiderpool agent ipam release metrics name
	ipam_release_total_counts                 = "ipam_release_total_counts"
	ipam_release_failure_counts               = "ipam_release_failure_counts"
	ipam_release_err_internal_counts          = "ipam_release_err_internal_counts"
	ipam_release_err_retries_exhausted_counts = "ipam_release_err_retries_exhausted_counts"

	ipam_release_average_duration_seconds = "ipam_release_average_duration_seconds"
	ipam_release_max_duration_seconds     = "ipam_release_max_duration_seconds"
	ipam_release_min_duration_seconds     = "ipam_release_min_duration_seconds"
	ipam_release_latest_duration_seconds  = "ipam_release_latest_duration_seconds"
	ipam_release_duration_seconds         = "ipam_release_duration_seconds_histogram"

	// spiderpool controller IP GC metrics name
	ip_gc_total_counts   = "ip_gc_total_counts"
	ip_gc_failure_counts = "ip_gc_failure_counts"

	subnet_ippool_counts = "subnet_ippool_counts"
)

var (
	// spiderpool agent ipam allocation metrics
	IpamAllocationTotalCounts               instrument.Int64Counter
	IpamAllocationFailureCounts             instrument.Int64Counter
	IpamAllocationRollbackFailureCounts     instrument.Int64Counter
	IpamAllocationErrInternalCounts         instrument.Int64Counter
	IpamAllocationErrNoAvailablePoolCounts  instrument.Int64Counter
	IpamAllocationErrRetriesExhaustedCounts instrument.Int64Counter
	IpamAllocationErrIPUsedOutCounts        instrument.Int64Counter

	ipamAllocationAverageDurationSeconds = new(asyncFloat64Gauge)
	ipamAllocationMaxDurationSeconds     = new(asyncFloat64Gauge)
	ipamAllocationMinDurationSeconds     = new(asyncFloat64Gauge)
	ipamAllocationLatestDurationSeconds  = new(asyncFloat64Gauge)
	ipamAllocationDurationSeconds        instrument.Float64Histogram

	// spiderpool agent ipam release metrics
	IpamReleaseTotalCounts               instrument.Int64Counter
	IpamReleaseFailureCounts             instrument.Int64Counter
	IpamReleaseErrInternalCounts         instrument.Int64Counter
	IpamReleaseErrRetriesExhaustedCounts instrument.Int64Counter

	ipamReleaseAverageDurationSeconds = new(asyncFloat64Gauge)
	ipamReleaseMaxDurationSeconds     = new(asyncFloat64Gauge)
	ipamReleaseMinDurationSeconds     = new(asyncFloat64Gauge)
	ipamReleaseLatestDurationSeconds  = new(asyncFloat64Gauge)
	ipamReleaseDurationSeconds        instrument.Float64Histogram

	// spiderpool controller IP GC metrics
	IPGCTotalCounts   instrument.Int64Counter
	IPGCFailureCounts instrument.Int64Counter

	SubnetPoolCounts = new(asyncInt64Gauge)
)

// asyncFloat64Gauge is custom otel float64 gauge
type asyncFloat64Gauge struct {
	gaugeMetric           instrument.Float64ObservableGauge
	observerValueToReport float64
	observerAttrsToReport []attribute.KeyValue
	observerLock          lock.RWMutex
}

// initGauge will new an otel float64 gauge metric and register a call back function
func (a *asyncFloat64Gauge) initGauge(metricName string, description string) error {
	tmpGauge, err := NewMetricFloat64Gauge(metricName, description)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool metric '%s', error: %v", metricName, err)
	}

	a.gaugeMetric = tmpGauge
	_, err = meter.RegisterCallback(func(_ context.Context, observer api.Observer) error {
		observer.ObserveFloat64(a.gaugeMetric,
			a.observerValueToReport,
			a.observerAttrsToReport...,
		)
		return nil
	}, a.gaugeMetric)
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool metric '%s', error: %v", metricName, err)
	}

	return nil
}

// Record uses otel async gauge observe function
func (a *asyncFloat64Gauge) Record(value float64, attrs ...attribute.KeyValue) {
	a.observerLock.Lock()
	a.observerValueToReport = value
	if len(attrs) != 0 {
		a.observerAttrsToReport = attrs
	}

	a.observerLock.Unlock()
}

// asyncInt64Gauge is custom otel int64 gauge
type asyncInt64Gauge struct {
	gaugeMetric           instrument.Int64ObservableGauge
	observerValueToReport int64
	observerAttrsToReport []attribute.KeyValue
	observerLock          lock.RWMutex
}

// initGauge will new an otel int64 gauge metric and register a call back function
func (a *asyncInt64Gauge) initGauge(metricName string, description string) error {
	tmpGauge, err := NewMetricInt64Gauge(metricName, description)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool metric '%s', error: %v", metricName, err)
	}

	a.gaugeMetric = tmpGauge
	_, err = meter.RegisterCallback(func(_ context.Context, observer api.Observer) error {
		observer.ObserveInt64(a.gaugeMetric,
			a.observerValueToReport,
			a.observerAttrsToReport...,
		)
		return nil
	}, a.gaugeMetric)
	if nil != err {
		return fmt.Errorf("failed to register callback for spiderpool metric '%s', error: %v", metricName, err)
	}

	return nil
}

// Record uses otel async gauge observe function
func (a *asyncInt64Gauge) Record(value int64, attrs ...attribute.KeyValue) {
	a.observerLock.Lock()
	a.observerValueToReport = value
	if len(attrs) != 0 {
		a.observerAttrsToReport = attrs
	}

	a.observerLock.Unlock()
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
	err = ipamAllocationAverageDurationSeconds.initGauge(ipam_allocation_average_duration_seconds, "spiderpool agent ipam average allocation duration")
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum allocation duration, metric type "float64 gauge"
	err = ipamAllocationMaxDurationSeconds.initGauge(ipam_allocation_max_duration_seconds, "spiderpool agent ipam maximum allocation duration")
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	err = ipamAllocationMinDurationSeconds.initGauge(ipam_allocation_min_duration_seconds, "spiderpool agent ipam minimum allocation duration")
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest allocation duration, metric type "float64 gauge"
	err = ipamAllocationLatestDurationSeconds.initGauge(ipam_allocation_latest_duration_seconds, "spiderpool agent ipam latest allocation duration")
	if nil != err {
		return err
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
	err = ipamReleaseAverageDurationSeconds.initGauge(ipam_release_average_duration_seconds, "spiderpool agent ipam average release duration")
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum release duration, metric type "float64 gauge"
	err = ipamReleaseMaxDurationSeconds.initGauge(ipam_release_max_duration_seconds, "spiderpool agent ipam maximum release duration")
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	err = ipamReleaseMinDurationSeconds.initGauge(ipam_release_min_duration_seconds, "spiderpool agent ipam minimum release duration")
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest release duration, metric type "float64 gauge"
	err = ipamReleaseLatestDurationSeconds.initGauge(ipam_release_latest_duration_seconds, "spiderpool agent ipam latest release duration")
	if nil != err {
		return err
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

	err = SubnetPoolCounts.initGauge(subnet_ippool_counts, "spider subnet corresponding ippools counts")
	if nil != err {
		return err
	}

	IPGCTotalCounts.Add(ctx, 0)
	IPGCFailureCounts.Add(ctx, 0)

	return nil
}
