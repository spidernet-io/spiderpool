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

const MetricPrefix = "spiderpool_"

const (
	// spiderpool agent ipam allocation metrics name
	ipam_allocation_counts                       = MetricPrefix + "ipam_allocation_counts"
	ipam_allocation_failure_counts               = MetricPrefix + "ipam_allocation_failure_counts"
	ipam_allocation_rollback_failure_counts      = MetricPrefix + "ipam_allocation_rollback_failure_counts"
	ipam_allocation_err_internal_counts          = MetricPrefix + "ipam_allocation_err_internal_counts"
	ipam_allocation_err_no_available_pool_counts = MetricPrefix + "ipam_allocation_err_no_available_pool_counts"
	ipam_allocation_err_retries_exhausted_counts = MetricPrefix + "ipam_allocation_err_retries_exhausted_counts"
	ipam_allocation_err_ip_used_out_counts       = MetricPrefix + "ipam_allocation_err_ip_used_out_counts"

	ipam_allocation_average_duration_seconds = MetricPrefix + "ipam_allocation_average_duration_seconds"
	ipam_allocation_max_duration_seconds     = MetricPrefix + "ipam_allocation_max_duration_seconds"
	ipam_allocation_min_duration_seconds     = MetricPrefix + "ipam_allocation_min_duration_seconds"
	ipam_allocation_latest_duration_seconds  = MetricPrefix + "ipam_allocation_latest_duration_seconds"
	ipam_allocation_duration_seconds         = MetricPrefix + "ipam_allocation_duration_seconds"

	// spiderpool agent ipam release metrics name
	ipam_release_counts                       = MetricPrefix + "ipam_release_counts"
	ipam_release_failure_counts               = MetricPrefix + "ipam_release_failure_counts"
	ipam_release_err_internal_counts          = MetricPrefix + "ipam_release_err_internal_counts"
	ipam_release_err_retries_exhausted_counts = MetricPrefix + "ipam_release_err_retries_exhausted_counts"

	ipam_release_average_duration_seconds = MetricPrefix + "ipam_release_average_duration_seconds"
	ipam_release_max_duration_seconds     = MetricPrefix + "ipam_release_max_duration_seconds"
	ipam_release_min_duration_seconds     = MetricPrefix + "ipam_release_min_duration_seconds"
	ipam_release_latest_duration_seconds  = MetricPrefix + "ipam_release_latest_duration_seconds"
	ipam_release_duration_seconds         = MetricPrefix + "ipam_release_duration_seconds"

	// spiderpool controller IP GC metrics name
	ip_gc_counts         = MetricPrefix + "ip_gc_counts"
	ip_gc_failure_counts = MetricPrefix + "ip_gc_failure_counts"

	subnet_ippool_counts = MetricPrefix + "subnet_ippool_counts"

	// spiderpool controller SpiderSubnet feature
	ippool_informer_conflict_counts             = MetricPrefix + "ippool_informer_conflict_counts"
	auto_pool_creation_average_duration_seconds = MetricPrefix + "auto_pool_creation_average_duration_seconds"
	auto_pool_creation_max_duration_seconds     = MetricPrefix + "auto_pool_creation_max_duration_seconds"
	auto_pool_creation_min_duration_seconds     = MetricPrefix + "auto_pool_creation_min_duration_seconds"
	auto_pool_creation_latest_duration_seconds  = MetricPrefix + "auto_pool_creation_latest_duration_seconds"
	auto_pool_creation_duration_seconds         = MetricPrefix + "auto_pool_creation_duration_seconds"
	auto_pool_scale_average_duration_seconds    = MetricPrefix + "auto_pool_scale_average_duration_seconds"
	auto_pool_scale_max_duration_seconds        = MetricPrefix + "auto_pool_scale_max_duration_seconds"
	auto_pool_scale_min_duration_seconds        = MetricPrefix + "auto_pool_scale_min_duration_seconds"
	auto_pool_scale_latest_duration_seconds     = MetricPrefix + "auto_pool_scale_latest_duration_seconds"
	auto_pool_scale_duration_seconds            = MetricPrefix + "auto_pool_scale_duration_seconds"
	auto_pool_scale_conflict_counts             = MetricPrefix + "auto_pool_scale_conflict_counts"
	auto_pool_waited_for_available_counts       = MetricPrefix + "auto_pool_waited_for_available_counts"
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
	ipamAllocationAverageDurationSeconds    = new(asyncFloat64Gauge)
	ipamAllocationMaxDurationSeconds        = new(asyncFloat64Gauge)
	ipamAllocationMinDurationSeconds        = new(asyncFloat64Gauge)
	ipamAllocationLatestDurationSeconds     = new(asyncFloat64Gauge)
	ipamAllocationDurationSecondsHistogram  instrument.Float64Histogram

	// spiderpool agent ipam release metrics
	IpamReleaseTotalCounts               instrument.Int64Counter
	IpamReleaseFailureCounts             instrument.Int64Counter
	IpamReleaseErrInternalCounts         instrument.Int64Counter
	IpamReleaseErrRetriesExhaustedCounts instrument.Int64Counter
	ipamReleaseAverageDurationSeconds    = new(asyncFloat64Gauge)
	ipamReleaseMaxDurationSeconds        = new(asyncFloat64Gauge)
	ipamReleaseMinDurationSeconds        = new(asyncFloat64Gauge)
	ipamReleaseLatestDurationSeconds     = new(asyncFloat64Gauge)
	ipamReleaseDurationSecondsHistogram  instrument.Float64Histogram

	// spiderpool controller IP GC metrics
	IPGCTotalCounts   instrument.Int64Counter
	IPGCFailureCounts instrument.Int64Counter

	SubnetPoolCounts = new(asyncInt64Gauge)

	// SpiderSubnet feature
	IPPoolInformerConflictCounts             instrument.Int64Counter
	autoPoolCreationAverageDurationSeconds   = new(asyncFloat64Gauge)
	autoPoolCreationMaxDurationSeconds       = new(asyncFloat64Gauge)
	autoPoolCreationMinDurationSeconds       = new(asyncFloat64Gauge)
	autoPoolCreationLatestDurationSeconds    = new(asyncFloat64Gauge)
	autoPoolCreationDurationSecondsHistogram instrument.Float64Histogram
	autoPoolScaleAverageDurationSeconds      = new(asyncFloat64Gauge)
	autoPoolScaleMaxDurationSeconds          = new(asyncFloat64Gauge)
	autoPoolScaleMinDurationSeconds          = new(asyncFloat64Gauge)
	autoPoolScaleLatestDurationSeconds       = new(asyncFloat64Gauge)
	autoPoolScaleDurationSecondsHistogram    instrument.Float64Histogram
	AutoPoolScaleConflictCounts              instrument.Int64Counter
	AutoPoolWaitedForAvailableCounts         instrument.Int64Counter
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

	err = initAutoPoolCreationMetrics(ctx)
	if nil != err {
		return err
	}

	autoPoolWaitedForAvailableCounts, err := NewMetricInt64Counter(auto_pool_waited_for_available_counts, "ipam waited for auto-created IPPool available counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", auto_pool_waited_for_available_counts, err)
	}
	AutoPoolWaitedForAvailableCounts = autoPoolWaitedForAvailableCounts
	AutoPoolWaitedForAvailableCounts.Add(ctx, 0)
	return nil
}

// InitSpiderpoolControllerMetrics serves for spiderpool-controller metrics initialization
func InitSpiderpoolControllerMetrics(ctx context.Context) error {
	err := initSpiderpoolControllerGCMetrics(ctx)
	if nil != err {
		return err
	}

	err = initAutoPoolCreationMetrics(ctx)
	if nil != err {
		return err
	}

	err = initAutoPoolScaleMetrics(ctx)
	if nil != err {
		return err
	}

	err = SubnetPoolCounts.initGauge(subnet_ippool_counts, "spider subnet corresponding ippools counts")
	if nil != err {
		return err
	}

	poolInformerConflictCounts, err := NewMetricInt64Counter(ippool_informer_conflict_counts, "ippool informer operation conflict counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ippool_informer_conflict_counts, err)
	}
	IPPoolInformerConflictCounts = poolInformerConflictCounts

	IPPoolInformerConflictCounts.Add(ctx, 0)

	return nil
}

// initSpiderpoolAgentAllocationMetrics will init spiderpool-agent IPAM allocation metrics
func initSpiderpoolAgentAllocationMetrics(ctx context.Context) error {
	// spiderpool agent ipam allocation total counts, metric type "int64 counter"
	allocationTotalCounts, err := NewMetricInt64Counter(ipam_allocation_counts, "spiderpool agent ipam allocation total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_counts, err)
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
	allocationHistogram, err := NewMetricFloat64Histogram(ipam_allocation_duration_seconds, "histogram of spiderpool agent ipam allocation duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_duration_seconds, err)
	}
	ipamAllocationDurationSecondsHistogram = allocationHistogram

	// set the spiderpool agent ipam allocation total counts initial data
	IpamAllocationTotalCounts.Add(ctx, 0)
	IpamAllocationFailureCounts.Add(ctx, 0)

	return nil
}

// initSpiderpoolAgentReleaseMetrics will init spiderpool-agent IPAM release metrics
func initSpiderpoolAgentReleaseMetrics(ctx context.Context) error {
	// spiderpool agent ipam release total counts, metric type "int64 counter"
	releaseTotalCounts, err := NewMetricInt64Counter(ipam_release_counts, "spiderpool agent ipam release total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_counts, err)
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
	releaseHistogram, err := NewMetricFloat64Histogram(ipam_release_duration_seconds, "histogram of spiderpool agent ipam release duration")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_duration_seconds, err)
	}
	ipamReleaseDurationSecondsHistogram = releaseHistogram

	// set the spiderpool agent ipam allocation total counts initial data
	IpamReleaseTotalCounts.Add(ctx, 0)
	IpamReleaseFailureCounts.Add(ctx, 0)

	return nil
}

// initSpiderpoolControllerGCMetrics will init spiderpool-controller IP gc metrics
func initSpiderpoolControllerGCMetrics(ctx context.Context) error {
	ipGCTotalCounts, err := NewMetricInt64Counter(ip_gc_counts, "spiderpool controller ip gc total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ip_gc_counts, err)
	}
	IPGCTotalCounts = ipGCTotalCounts

	ipGCFailureCounts, err := NewMetricInt64Counter(ip_gc_failure_counts, "spiderpool controller ip gc total counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ip_gc_failure_counts, err)
	}
	IPGCFailureCounts = ipGCFailureCounts

	IPGCTotalCounts.Add(ctx, 0)
	IPGCFailureCounts.Add(ctx, 0)

	return nil
}

// initAutoPoolCreationMetrics will init auto-created IPPool creation metrics
// Notice: this metrics serve for both Spiderpool-agent and Spiderpool-controller components
func initAutoPoolCreationMetrics(ctx context.Context) error {
	err := autoPoolCreationAverageDurationSeconds.initGauge(auto_pool_creation_average_duration_seconds, "auto-created IPPool creation average duration")
	if nil != err {
		return err
	}

	err = autoPoolCreationMaxDurationSeconds.initGauge(auto_pool_creation_max_duration_seconds, "auto-created IPPool creation max duration")
	if nil != err {
		return err
	}

	err = autoPoolCreationMinDurationSeconds.initGauge(auto_pool_creation_min_duration_seconds, "auto-created IPPool creation min duration")
	if nil != err {
		return err
	}

	err = autoPoolCreationLatestDurationSeconds.initGauge(auto_pool_creation_latest_duration_seconds, "auto-created IPPool creation latest duration")
	if nil != err {
		return err
	}

	autoPoolCreationHistogram, err := NewMetricFloat64Histogram(auto_pool_creation_duration_seconds, "histogram of auto-created IPPool creation duration")
	if nil != err {
		return err
	}
	autoPoolCreationDurationSecondsHistogram = autoPoolCreationHistogram

	return nil
}

// initAutoPoolScaleMetrics will init spiderpool-controller IPPool informer auto-created IPPool scale metrics
func initAutoPoolScaleMetrics(ctx context.Context) error {
	err := autoPoolScaleAverageDurationSeconds.initGauge(auto_pool_scale_average_duration_seconds, "auto-created IPPool scale average duration")
	if nil != err {
		return err
	}

	err = autoPoolScaleMaxDurationSeconds.initGauge(auto_pool_scale_max_duration_seconds, "auto-created IPPool scale max duration")
	if nil != err {
		return err
	}

	err = autoPoolScaleMinDurationSeconds.initGauge(auto_pool_scale_min_duration_seconds, "auto-created IPPool scale min duration")
	if nil != err {
		return err
	}

	err = autoPoolScaleLatestDurationSeconds.initGauge(auto_pool_scale_latest_duration_seconds, "auto-created IPPool scale latest duration")
	if nil != err {
		return err
	}

	autoPoolScaleHistogram, err := NewMetricFloat64Histogram(auto_pool_scale_duration_seconds, "histogram of auto-created IPPool scale duration")
	if nil != err {
		return fmt.Errorf("")
	}
	autoPoolScaleDurationSecondsHistogram = autoPoolScaleHistogram

	autoPoolScaleConflictCounts, err := NewMetricInt64Counter(auto_pool_scale_conflict_counts, "scale auto-created IPPool conflict counts")
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", auto_pool_scale_conflict_counts, err)
	}
	AutoPoolScaleConflictCounts = autoPoolScaleConflictCounts

	AutoPoolScaleConflictCounts.Add(ctx, 0)

	return nil
}
