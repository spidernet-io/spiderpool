// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/rdmametrics"
)

const metricPrefix = "spiderpool_"
const debugPrefix = "debug_"

const (
	// spiderpool agent ipam allocation metrics name
	ipam_allocation_counts                        = metricPrefix + "ipam_allocation_counts"
	ipam_allocation_failure_counts                = metricPrefix + "ipam_allocation_failure_counts"
	ipam_allocation_update_ippool_conflict_counts = metricPrefix + "ipam_allocation_update_ippool_conflict_counts"
	ipam_allocation_err_internal_counts           = metricPrefix + "ipam_allocation_err_internal_counts"
	ipam_allocation_err_no_available_pool_counts  = metricPrefix + "ipam_allocation_err_no_available_pool_counts"
	ipam_allocation_err_retries_exhausted_counts  = metricPrefix + "ipam_allocation_err_retries_exhausted_counts"
	ipam_allocation_err_ip_used_out_counts        = metricPrefix + "ipam_allocation_err_ip_used_out_counts"

	ipam_allocation_average_duration_seconds = metricPrefix + "ipam_allocation_average_duration_seconds"
	ipam_allocation_max_duration_seconds     = metricPrefix + "ipam_allocation_max_duration_seconds"
	ipam_allocation_min_duration_seconds     = metricPrefix + "ipam_allocation_min_duration_seconds"
	ipam_allocation_latest_duration_seconds  = metricPrefix + "ipam_allocation_latest_duration_seconds"
	ipam_allocation_duration_seconds         = metricPrefix + "ipam_allocation_duration_seconds"

	ipam_allocation_average_limit_duration_seconds = metricPrefix + "ipam_allocation_average_limit_duration_seconds"
	ipam_allocation_max_limit_duration_seconds     = metricPrefix + "ipam_allocation_max_limit_duration_seconds"
	ipam_allocation_min_limit_duration_seconds     = metricPrefix + "ipam_allocation_min_limit_duration_seconds"
	ipam_allocation_latest_limit_duration_seconds  = metricPrefix + "ipam_allocation_latest_limit_duration_seconds"
	ipam_allocation_limit_duration_seconds         = metricPrefix + "ipam_allocation_limit_duration_seconds"

	// spiderpool agent ipam release metrics name
	ipam_release_counts                        = metricPrefix + "ipam_release_counts"
	ipam_release_failure_counts                = metricPrefix + "ipam_release_failure_counts"
	ipam_release_update_ippool_conflict_counts = metricPrefix + "ipam_release_update_ippool_conflict_counts"
	ipam_release_err_internal_counts           = metricPrefix + "ipam_release_err_internal_counts"
	ipam_release_err_retries_exhausted_counts  = metricPrefix + "ipam_release_err_retries_exhausted_counts"

	ipam_release_average_duration_seconds = metricPrefix + "ipam_release_average_duration_seconds"
	ipam_release_max_duration_seconds     = metricPrefix + "ipam_release_max_duration_seconds"
	ipam_release_min_duration_seconds     = metricPrefix + "ipam_release_min_duration_seconds"
	ipam_release_latest_duration_seconds  = metricPrefix + "ipam_release_latest_duration_seconds"
	ipam_release_duration_seconds         = metricPrefix + "ipam_release_duration_seconds"

	ipam_release_average_limit_duration_seconds = metricPrefix + "ipam_release_average_limit_duration_seconds"
	ipam_release_max_limit_duration_seconds     = metricPrefix + "ipam_release_max_limit_duration_seconds"
	ipam_release_min_limit_duration_seconds     = metricPrefix + "ipam_release_min_limit_duration_seconds"
	ipam_release_latest_limit_duration_seconds  = metricPrefix + "ipam_release_latest_limit_duration_seconds"
	ipam_release_limit_duration_seconds         = metricPrefix + "ipam_release_limit_duration_seconds"

	// spiderpool controller IP GC metrics name
	ip_gc_counts         = metricPrefix + "ip_gc_counts"
	ip_gc_failure_counts = metricPrefix + "ip_gc_failure_counts"

	// spiderpool IPPool and Subnet metrics and these include some debug level metrics
	total_ippool_counts                   = metricPrefix + "total_ippool_counts"
	ippool_total_ip_counts                = metricPrefix + debugPrefix + "ippool_total_ip_counts"
	ippool_available_ip_counts            = metricPrefix + debugPrefix + "ippool_available_ip_counts"
	total_subnet_counts                   = metricPrefix + "total_subnet_counts"
	subnet_ippool_counts                  = metricPrefix + debugPrefix + "subnet_ippool_counts"
	subnet_total_ip_counts                = metricPrefix + debugPrefix + "subnet_total_ip_counts"
	subnet_available_ip_counts            = metricPrefix + debugPrefix + "subnet_available_ip_counts"
	auto_pool_waited_for_available_counts = metricPrefix + debugPrefix + "auto_pool_waited_for_available_counts"
)

var (
	// ipam allocation metrics in spiderpool-agent
	IpamAllocationTotalCounts                   api.Int64Counter
	IpamAllocationFailureCounts                 api.Int64Counter
	IpamAllocationUpdateIPPoolConflictCounts    api.Int64Counter
	IpamAllocationErrInternalCounts             api.Int64Counter
	IpamAllocationErrNoAvailablePoolCounts      api.Int64Counter
	IpamAllocationErrRetriesExhaustedCounts     api.Int64Counter
	IpamAllocationErrIPUsedOutCounts            api.Int64Counter
	ipamAllocationAverageDurationSeconds        = new(asyncFloat64Gauge)
	ipamAllocationMaxDurationSeconds            = new(asyncFloat64Gauge)
	ipamAllocationMinDurationSeconds            = new(asyncFloat64Gauge)
	ipamAllocationLatestDurationSeconds         = new(asyncFloat64Gauge)
	ipamAllocationDurationSecondsHistogram      api.Float64Histogram
	ipamAllocationAverageLimitDurationSeconds   = new(asyncFloat64Gauge)
	ipamAllocationMaxLimitDurationSeconds       = new(asyncFloat64Gauge)
	ipamAllocationMinLimitDurationSeconds       = new(asyncFloat64Gauge)
	ipamAllocationLatestLimitDurationSeconds    = new(asyncFloat64Gauge)
	ipamAllocationLimitDurationSecondsHistogram api.Float64Histogram

	// ipam release metrics in spiderpool-agent
	IpamReleaseTotalCounts                   api.Int64Counter
	IpamReleaseFailureCounts                 api.Int64Counter
	IpamReleaseUpdateIPPoolConflictCounts    api.Int64Counter
	IpamReleaseErrInternalCounts             api.Int64Counter
	IpamReleaseErrRetriesExhaustedCounts     api.Int64Counter
	ipamReleaseAverageDurationSeconds        = new(asyncFloat64Gauge)
	ipamReleaseMaxDurationSeconds            = new(asyncFloat64Gauge)
	ipamReleaseMinDurationSeconds            = new(asyncFloat64Gauge)
	ipamReleaseLatestDurationSeconds         = new(asyncFloat64Gauge)
	ipamReleaseDurationSecondsHistogram      api.Float64Histogram
	ipamReleaseAverageLimitDurationSeconds   = new(asyncFloat64Gauge)
	ipamReleaseMaxLimitDurationSeconds       = new(asyncFloat64Gauge)
	ipamReleaseMinLimitDurationSeconds       = new(asyncFloat64Gauge)
	ipamReleaseLatestLimitDurationSeconds    = new(asyncFloat64Gauge)
	ipamReleaseLimitDurationSecondsHistogram api.Float64Histogram

	// IP GC metrics in spiderpool-controller
	IPGCTotalCounts   api.Int64Counter
	IPGCFailureCounts api.Int64Counter

	// IPPool&Subnet metrics in spiderpool-controller
	TotalIPPoolCounts       = new(asyncInt64Gauge)
	IPPoolTotalIPCounts     api.Int64Counter
	IPPoolAvailableIPCounts api.Int64Counter
	TotalSubnetCounts       = new(asyncInt64Gauge)
	SubnetPoolCounts        = new(asyncInt64Gauge)
	SubnetTotalIPCounts     api.Int64Counter
	SubnetAvailableIPCounts api.Int64Counter

	// SpiderSubnet feature performance monitoring metric in spiderpool-agent
	AutoPoolWaitedForAvailableCounts api.Int64Counter
)

// asyncFloat64Gauge is custom otel float64 gauge
type asyncFloat64Gauge struct {
	gaugeMetric           api.Float64ObservableGauge
	observerValueToReport float64
	observerAttrsToReport []attribute.KeyValue
	observerLock          lock.RWMutex
}

// initGauge will new an otel float64 gauge metric and register a call back function
func (a *asyncFloat64Gauge) initGauge(metricName string, description string, isDebugLevel bool) error {
	m := meter
	if isDebugLevel {
		m = debugLevelMeter
	}

	tmpGauge, err := newMetricFloat64Gauge(metricName, description, isDebugLevel)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool metric '%s', error: %v", metricName, err)
	}

	a.gaugeMetric = tmpGauge
	_, err = m.RegisterCallback(func(_ context.Context, observer api.Observer) error {
		observer.ObserveFloat64(a.gaugeMetric,
			a.observerValueToReport,
			api.WithAttributes(a.observerAttrsToReport...),
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
	gaugeMetric           api.Int64ObservableGauge
	observerValueToReport int64
	observerAttrsToReport []attribute.KeyValue
	observerLock          lock.RWMutex
}

// initGauge will new an otel int64 gauge metric and register a call back function
func (a *asyncInt64Gauge) initGauge(metricName string, description string, isDebugLevel bool) error {
	m := meter
	if isDebugLevel {
		m = debugLevelMeter
	}

	tmpGauge, err := newMetricInt64Gauge(metricName, description, isDebugLevel)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool metric '%s', error: %v", metricName, err)
	}

	a.gaugeMetric = tmpGauge
	_, err = m.RegisterCallback(func(_ context.Context, observer api.Observer) error {
		observer.ObserveInt64(a.gaugeMetric,
			a.observerValueToReport,
			api.WithAttributes(a.observerAttrsToReport...),
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
func InitSpiderpoolAgentMetrics(ctx context.Context, enableRDMAMetric bool, client client.Client) error {
	// for rdma
	if enableRDMAMetric {
		err := rdmametrics.Register(ctx, meter, client)
		if err != nil {
			return err
		}
	}

	err := initSpiderpoolAgentAllocationMetrics(ctx)
	if nil != err {
		return err
	}

	err = initSpiderpoolAgentReleaseMetrics(ctx)
	if nil != err {
		return err
	}

	autoPoolWaitedForAvailableCounts, err := newMetricInt64Counter(auto_pool_waited_for_available_counts, "ipam waited for auto-created IPPool available counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", auto_pool_waited_for_available_counts, err)
	}
	AutoPoolWaitedForAvailableCounts = autoPoolWaitedForAvailableCounts

	return nil
}

// InitSpiderpoolControllerMetrics serves for spiderpool-controller metrics initialization
func InitSpiderpoolControllerMetrics(ctx context.Context) error {
	err := initSpiderpoolControllerGCMetrics(ctx)
	if nil != err {
		return err
	}

	err = initSpiderpoolControllerCRMetrics(ctx)
	if nil != err {
		return err
	}

	return nil
}

// initSpiderpoolAgentAllocationMetrics will init spiderpool-agent IPAM allocation metrics
func initSpiderpoolAgentAllocationMetrics(ctx context.Context) error {
	// spiderpool agent ipam allocation total counts, metric type "int64 counter"
	allocationTotalCounts, err := newMetricInt64Counter(ipam_allocation_counts, "spiderpool agent ipam allocation total counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_counts, err)
	}
	IpamAllocationTotalCounts = allocationTotalCounts
	IpamAllocationTotalCounts.Add(ctx, 0)

	// spiderpool agent ipam allocation failure counts, metric type "int64 counter"
	allocationFailureCounts, err := newMetricInt64Counter(ipam_allocation_failure_counts, "spiderpool agent ipam allocation failure counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_failure_counts, err)
	}
	IpamAllocationFailureCounts = allocationFailureCounts
	IpamAllocationFailureCounts.Add(ctx, 0)

	// spiderpool agent ipam allocation update IPPool conflict counts, metric type "int64 counter"
	allocationUpdateIPPoolConflictCounts, err := newMetricInt64Counter(ipam_allocation_update_ippool_conflict_counts, "spiderpool agent ipam allocation update IPPool conflict counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_update_ippool_conflict_counts, err)
	}
	IpamAllocationUpdateIPPoolConflictCounts = allocationUpdateIPPoolConflictCounts

	// spiderpool agent ipam allocation internal error counts, metric type "int64 counter"
	allocationErrInternalCounts, err := newMetricInt64Counter(ipam_allocation_err_internal_counts, "spiderpool agent ipam allocation internal error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_err_internal_counts, err)
	}
	IpamAllocationErrInternalCounts = allocationErrInternalCounts

	// spiderpool agent ipam allocation no available IPPool error counts, metric type "int64 counter"
	allocationErrNoAvailablePoolCounts, err := newMetricInt64Counter(ipam_allocation_err_no_available_pool_counts, "spiderpool agent ipam allocation no available IPPool error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_err_no_available_pool_counts, err)
	}
	IpamAllocationErrNoAvailablePoolCounts = allocationErrNoAvailablePoolCounts

	// spiderpool agent ipam allocation retries exhausted error counts, metric type "int64 counter"
	allocationErrRetriesExhaustedCounts, err := newMetricInt64Counter(ipam_allocation_err_retries_exhausted_counts, "spiderpool agent ipam allocation retries exhausted error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_err_retries_exhausted_counts, err)
	}
	IpamAllocationErrRetriesExhaustedCounts = allocationErrRetriesExhaustedCounts

	// spiderpool agent ipam allocation IP addresses used out error counts, metric type "int64 counter"
	allocationErrIPUsedOutCounts, err := newMetricInt64Counter(ipam_allocation_err_ip_used_out_counts, "spiderpool agent ipam allocation IP addresses used out error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_err_ip_used_out_counts, err)
	}
	IpamAllocationErrIPUsedOutCounts = allocationErrIPUsedOutCounts

	// spiderpool agent ipam average allocation duration, metric type "float64 gauge"
	err = ipamAllocationAverageDurationSeconds.initGauge(ipam_allocation_average_duration_seconds, "spiderpool agent ipam average allocation duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum allocation duration, metric type "float64 gauge"
	err = ipamAllocationMaxDurationSeconds.initGauge(ipam_allocation_max_duration_seconds, "spiderpool agent ipam maximum allocation duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	err = ipamAllocationMinDurationSeconds.initGauge(ipam_allocation_min_duration_seconds, "spiderpool agent ipam minimum allocation duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest allocation duration, metric type "float64 gauge"
	err = ipamAllocationLatestDurationSeconds.initGauge(ipam_allocation_latest_duration_seconds, "spiderpool agent ipam latest allocation duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam allocation duration bucket, metric type "float64 histogram"
	allocationHistogram, err := newMetricFloat64Histogram(ipam_allocation_duration_seconds, "histogram of spiderpool agent ipam allocation duration", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_duration_seconds, err)
	}
	ipamAllocationDurationSecondsHistogram = allocationHistogram

	// spiderpool agent ipam average allocation limit duration, metric type "float64 gauge"
	err = ipamAllocationAverageLimitDurationSeconds.initGauge(ipam_allocation_average_limit_duration_seconds, "spiderpool agent ipam average allocation limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum allocation limit duration, metric type "float64 gauge"
	err = ipamAllocationMaxLimitDurationSeconds.initGauge(ipam_allocation_max_limit_duration_seconds, "spiderpool agent ipam maximum allocation limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation limit duration, metric type "float64 gauge"
	err = ipamAllocationMinLimitDurationSeconds.initGauge(ipam_allocation_min_limit_duration_seconds, "spiderpool agent ipam minimum allocation limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest allocation limit duration, metric type "float64 gauge"
	err = ipamAllocationLatestLimitDurationSeconds.initGauge(ipam_allocation_latest_limit_duration_seconds, "spiderpool agent ipam latest allocation limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam allocation limit duration bucket, metric type "float64 histogram"
	allocationLimitHistogram, err := newMetricFloat64Histogram(ipam_allocation_limit_duration_seconds, "histogram of spiderpool agent ipam allocation limit duration", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_allocation_limit_duration_seconds, err)
	}
	ipamAllocationLimitDurationSecondsHistogram = allocationLimitHistogram

	return nil
}

// initSpiderpoolAgentReleaseMetrics will init spiderpool-agent IPAM release metrics
func initSpiderpoolAgentReleaseMetrics(ctx context.Context) error {
	// spiderpool agent ipam release total counts, metric type "int64 counter"
	releaseTotalCounts, err := newMetricInt64Counter(ipam_release_counts, "spiderpool agent ipam release total counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_counts, err)
	}
	IpamReleaseTotalCounts = releaseTotalCounts
	IpamReleaseTotalCounts.Add(ctx, 0)

	// spiderpool agent ipam release failure counts, metric type "int64 counter"
	releaseFailureCounts, err := newMetricInt64Counter(ipam_release_failure_counts, "spiderpool agent ipam release failure counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_failure_counts, err)
	}
	IpamReleaseFailureCounts = releaseFailureCounts
	IpamReleaseFailureCounts.Add(ctx, 0)

	// spiderpool agent ipam release update IPPool conflict counts, metric type "int64 counter"
	releaseUpdateIPPoolConflictCounts, err := newMetricInt64Counter(ipam_release_update_ippool_conflict_counts, "spiderpool agent ipam release update IPPool conflict counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_update_ippool_conflict_counts, err)
	}
	IpamReleaseUpdateIPPoolConflictCounts = releaseUpdateIPPoolConflictCounts

	// spiderpool agent ipam releasing internal error counts, metric type "int64 counter"
	releasingErrInternalCounts, err := newMetricInt64Counter(ipam_release_err_internal_counts, "spiderpool agent ipam release internal error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_err_internal_counts, err)
	}
	IpamReleaseErrInternalCounts = releasingErrInternalCounts

	// spiderpool agent ipam releasing retries exhausted error counts, metric type "int64 counter"
	releasingErrRetriesExhaustedCounts, err := newMetricInt64Counter(ipam_release_err_retries_exhausted_counts, "spiderpool agent ipam release retries exhausted error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_err_retries_exhausted_counts, err)
	}
	IpamReleaseErrRetriesExhaustedCounts = releasingErrRetriesExhaustedCounts

	// spiderpool agent ipam average release duration, metric type "float64 gauge"
	err = ipamReleaseAverageDurationSeconds.initGauge(ipam_release_average_duration_seconds, "spiderpool agent ipam average release duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum release duration, metric type "float64 gauge"
	err = ipamReleaseMaxDurationSeconds.initGauge(ipam_release_max_duration_seconds, "spiderpool agent ipam maximum release duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	err = ipamReleaseMinDurationSeconds.initGauge(ipam_release_min_duration_seconds, "spiderpool agent ipam minimum release duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest release duration, metric type "float64 gauge"
	err = ipamReleaseLatestDurationSeconds.initGauge(ipam_release_latest_duration_seconds, "spiderpool agent ipam latest release duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam allocation duration bucket, metric type "float64 histogram"
	releaseHistogram, err := newMetricFloat64Histogram(ipam_release_duration_seconds, "histogram of spiderpool agent ipam release duration", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_duration_seconds, err)
	}
	ipamReleaseDurationSecondsHistogram = releaseHistogram

	// spiderpool agent ipam average release limit duration, metric type "float64 gauge"
	err = ipamReleaseAverageLimitDurationSeconds.initGauge(ipam_release_average_limit_duration_seconds, "spiderpool agent ipam average release limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum release limit duration, metric type "float64 gauge"
	err = ipamReleaseMaxLimitDurationSeconds.initGauge(ipam_release_max_limit_duration_seconds, "spiderpool agent ipam maximum release limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation limit duration, metric type "float64 gauge"
	err = ipamReleaseMinLimitDurationSeconds.initGauge(ipam_release_min_limit_duration_seconds, "spiderpool agent ipam minimum release limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest release limit duration, metric type "float64 gauge"
	err = ipamReleaseLatestLimitDurationSeconds.initGauge(ipam_release_latest_limit_duration_seconds, "spiderpool agent ipam latest release limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam allocation limit duration bucket, metric type "float64 histogram"
	releaseLimitHistogram, err := newMetricFloat64Histogram(ipam_release_limit_duration_seconds, "histogram of spiderpool agent ipam release limit duration", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_limit_duration_seconds, err)
	}
	ipamReleaseLimitDurationSecondsHistogram = releaseLimitHistogram

	return nil
}

// initSpiderpoolControllerGCMetrics will init spiderpool-controller IP gc metrics
func initSpiderpoolControllerGCMetrics(ctx context.Context) error {
	ipGCTotalCounts, err := newMetricInt64Counter(ip_gc_counts, "spiderpool controller ip gc total counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ip_gc_counts, err)
	}
	IPGCTotalCounts = ipGCTotalCounts
	IPGCTotalCounts.Add(ctx, 0)

	ipGCFailureCounts, err := newMetricInt64Counter(ip_gc_failure_counts, "spiderpool controller ip gc total counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ip_gc_failure_counts, err)
	}
	IPGCFailureCounts = ipGCFailureCounts
	ipGCFailureCounts.Add(ctx, 0)

	releaseUpdateIPPoolConflictCounts, err := newMetricInt64Counter(ipam_release_update_ippool_conflict_counts, "spiderpool controller gc release update IPPool conflict counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %v", ipam_release_update_ippool_conflict_counts, err)
	}
	IpamReleaseUpdateIPPoolConflictCounts = releaseUpdateIPPoolConflictCounts

	return nil
}

func initSpiderpoolControllerCRMetrics(ctx context.Context) error {
	err := TotalSubnetCounts.initGauge(total_subnet_counts, "spiderpool total SpiderSubnet counts", false)
	if nil != err {
		return err
	}

	subnetTotalIPCounts, err := newMetricInt64Counter(subnet_total_ip_counts, "spiderpool single SpiderSubnet corresponding total IP counts", true)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", subnet_total_ip_counts, err)
	}
	SubnetTotalIPCounts = subnetTotalIPCounts

	subnetAvailableIPCounts, err := newMetricInt64Counter(subnet_available_ip_counts, "spiderpool single SpiderSubnet corresponding available IP counts", true)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", subnet_available_ip_counts, err)
	}
	SubnetAvailableIPCounts = subnetAvailableIPCounts

	err = TotalIPPoolCounts.initGauge(total_ippool_counts, "spiderpool total SpiderIPPool counts", false)
	if nil != err {
		return err
	}

	poolTotalIPCounts, err := newMetricInt64Counter(ippool_total_ip_counts, "spiderpool single SpiderIPPool corresponding total IP counts", true)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ippool_total_ip_counts, err)
	}
	IPPoolTotalIPCounts = poolTotalIPCounts

	poolAvailableIPCounts, err := newMetricInt64Counter(ippool_available_ip_counts, "spiderpool single SpiderIPPool corresponding available IP counts", true)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %v", ippool_available_ip_counts, err)
	}
	IPPoolAvailableIPCounts = poolAvailableIPCounts

	err = SubnetPoolCounts.initGauge(subnet_ippool_counts, "spider subnet corresponding ippools counts", true)
	if nil != err {
		return err
	}

	return nil
}
