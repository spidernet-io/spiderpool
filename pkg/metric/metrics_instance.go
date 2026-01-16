// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	api "go.opentelemetry.io/otel/metric"

	"github.com/spidernet-io/spiderpool/pkg/lock"
	"github.com/spidernet-io/spiderpool/pkg/podownercache"
	"github.com/spidernet-io/spiderpool/pkg/rdmametrics"
)

const (
	metricPrefix = "spiderpool_"
	debugPrefix  = "debug_"
)

const (
	// spiderpool agent ipam allocation metrics name
	ipamAllocationCountsName                     = metricPrefix + "ipamAllocationCountsName"
	ipamAllocationFailureCountsName              = metricPrefix + "ipamAllocationFailureCountsName"
	ipamAllocationUpdateIPPoolConflictCountsName = metricPrefix + "ipamAllocationUpdateIPPoolConflictCountsName"
	ipamAllocationErrInternalCountsName          = metricPrefix + "ipamAllocationErrInternalCountsName"
	ipamAllocationErrNoAvailablePoolCountsName   = metricPrefix + "ipamAllocationErrNoAvailablePoolCountsName"
	ipamAllocationErrRetriesExhaustedCountsName  = metricPrefix + "ipamAllocationErrRetriesExhaustedCountsName"
	ipamAllocationErrIPUsedOutCountsName         = metricPrefix + "ipamAllocationErrIPUsedOutCountsName"

	ipamAllocationAverageDurationSecondsName = metricPrefix + "ipamAllocationAverageDurationSecondsName"
	ipamAllocationMaxDurationSecondsName     = metricPrefix + "ipamAllocationMaxDurationSecondsName"
	ipamAllocationMinDurationSecondsName     = metricPrefix + "ipamAllocationMinDurationSecondsName"
	ipamAllocationLatestDurationSecondsName  = metricPrefix + "ipamAllocationLatestDurationSecondsName"
	ipamAllocationDurationSecondsName        = metricPrefix + "ipamAllocationDurationSecondsName"

	ipamAllocationAverageLimitDurationSecondsName = metricPrefix + "ipamAllocationAverageLimitDurationSecondsName"
	ipamAllocationMaxLimitDurationSecondsName     = metricPrefix + "ipamAllocationMaxLimitDurationSecondsName"
	ipamAllocationMinLimitDurationSecondsName     = metricPrefix + "ipamAllocationMinLimitDurationSecondsName"
	ipamAllocationLatestLimitDurationSecondsName  = metricPrefix + "ipamAllocationLatestLimitDurationSecondsName"
	ipamAllocationLimitDurationSecondsName        = metricPrefix + "ipamAllocationLimitDurationSecondsName"

	// spiderpool agent ipam release metrics name
	ipamReleaseCountsName                     = metricPrefix + "ipamReleaseCountsName"
	ipamReleaseFailureCountsName              = metricPrefix + "ipamReleaseFailureCountsName"
	ipamReleaseUpdateIPPoolConflictCountsName = metricPrefix + "ipamReleaseUpdateIPPoolConflictCountsName"
	ipamReleaseErrInternalCountsName          = metricPrefix + "ipamReleaseErrInternalCountsName"
	ipamReleaseErrRetriesExhaustedCountsName  = metricPrefix + "ipamReleaseErrRetriesExhaustedCountsName"

	ipamReleaseAverageDurationSecondsName = metricPrefix + "ipamReleaseAverageDurationSecondsName"
	ipamReleaseMaxDurationSecondsName     = metricPrefix + "ipamReleaseMaxDurationSecondsName"
	ipamReleaseMinDurationSecondsName     = metricPrefix + "ipamReleaseMinDurationSecondsName"
	ipamReleaseLatestDurationSecondsName  = metricPrefix + "ipamReleaseLatestDurationSecondsName"
	ipamReleaseDurationSecondsName        = metricPrefix + "ipamReleaseDurationSecondsName"

	ipamReleaseAverageLimitDurationSecondsName = metricPrefix + "ipamReleaseAverageLimitDurationSecondsName"
	ipamReleaseMaxLimitDurationSecondsName     = metricPrefix + "ipamReleaseMaxLimitDurationSecondsName"
	ipamReleaseMinLimitDurationSecondsName     = metricPrefix + "ipamReleaseMinLimitDurationSecondsName"
	ipamReleaseLatestLimitDurationSecondsName  = metricPrefix + "ipamReleaseLatestLimitDurationSecondsName"
	ipamReleaseLimitDurationSecondsName        = metricPrefix + "ipamReleaseLimitDurationSecondsName"

	// spiderpool controller IP GC metrics name
	ipGCCCountsName       = metricPrefix + "ipGCCCountsName"
	ipGCFailureCountsName = metricPrefix + "ipGCFailureCountsName"

	// spiderpool IPPool and Subnet metrics and these include some debug level metrics
	totalIPPoolCountsName                = metricPrefix + "totalIPPoolCountsName"
	ippoolTotalIPCountsName              = metricPrefix + debugPrefix + "ippoolTotalIPCountsName"
	ippoolAvailableIPCountsName          = metricPrefix + debugPrefix + "ippoolAvailableIPCountsName"
	totalSubnetCountsName                = metricPrefix + "totalSubnetCountsName"
	subnetIPPoolCountsName               = metricPrefix + debugPrefix + "subnetIPPoolCountsName"
	subnetTotalIPCountsName              = metricPrefix + debugPrefix + "subnetTotalIPCountsName"
	subnetAvailableIPCountsName          = metricPrefix + debugPrefix + "subnetAvailableIPCountsName"
	autoPoolWaitedForAvailableCountsName = metricPrefix + debugPrefix + "autoPoolWaitedForAvailableCountsName"
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
		return fmt.Errorf("failed to new spiderpool metric '%s', error: %w", metricName, err)
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
		return fmt.Errorf("failed to register callback for spiderpool metric '%s', error: %w", metricName, err)
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
		return fmt.Errorf("failed to new spiderpool metric '%s', error: %w", metricName, err)
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
		return fmt.Errorf("failed to register callback for spiderpool metric '%s', error: %w", metricName, err)
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
func InitSpiderpoolAgentMetrics(ctx context.Context, cache podownercache.CacheInterface) error {
	// for rdma
	if cache != nil {
		err := rdmametrics.Register(ctx, meter, cache)
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

	autoPoolWaitedForAvailableCounts, err := newMetricInt64Counter(autoPoolWaitedForAvailableCountsName, "ipam waited for auto-created IPPool available counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", autoPoolWaitedForAvailableCountsName, err)
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
	allocationTotalCounts, err := newMetricInt64Counter(ipamAllocationCountsName, "spiderpool agent ipam allocation total counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationCountsName, err)
	}
	IpamAllocationTotalCounts = allocationTotalCounts
	IpamAllocationTotalCounts.Add(ctx, 0)

	// spiderpool agent ipam allocation failure counts, metric type "int64 counter"
	allocationFailureCounts, err := newMetricInt64Counter(ipamAllocationFailureCountsName, "spiderpool agent ipam allocation failure counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationFailureCountsName, err)
	}
	IpamAllocationFailureCounts = allocationFailureCounts
	IpamAllocationFailureCounts.Add(ctx, 0)

	// spiderpool agent ipam allocation update IPPool conflict counts, metric type "int64 counter"
	allocationUpdateIPPoolConflictCounts, err := newMetricInt64Counter(ipamAllocationUpdateIPPoolConflictCountsName, "spiderpool agent ipam allocation update IPPool conflict counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationUpdateIPPoolConflictCountsName, err)
	}
	IpamAllocationUpdateIPPoolConflictCounts = allocationUpdateIPPoolConflictCounts

	// spiderpool agent ipam allocation internal error counts, metric type "int64 counter"
	allocationErrInternalCounts, err := newMetricInt64Counter(ipamAllocationErrInternalCountsName, "spiderpool agent ipam allocation internal error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationErrInternalCountsName, err)
	}
	IpamAllocationErrInternalCounts = allocationErrInternalCounts

	// spiderpool agent ipam allocation no available IPPool error counts, metric type "int64 counter"
	allocationErrNoAvailablePoolCounts, err := newMetricInt64Counter(ipamAllocationErrNoAvailablePoolCountsName, "spiderpool agent ipam allocation no available IPPool error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationErrNoAvailablePoolCountsName, err)
	}
	IpamAllocationErrNoAvailablePoolCounts = allocationErrNoAvailablePoolCounts

	// spiderpool agent ipam allocation retries exhausted error counts, metric type "int64 counter"
	allocationErrRetriesExhaustedCounts, err := newMetricInt64Counter(ipamAllocationErrRetriesExhaustedCountsName, "spiderpool agent ipam allocation retries exhausted error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationErrRetriesExhaustedCountsName, err)
	}
	IpamAllocationErrRetriesExhaustedCounts = allocationErrRetriesExhaustedCounts

	// spiderpool agent ipam allocation IP addresses used out error counts, metric type "int64 counter"
	allocationErrIPUsedOutCounts, err := newMetricInt64Counter(ipamAllocationErrIPUsedOutCountsName, "spiderpool agent ipam allocation IP addresses used out error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationErrIPUsedOutCountsName, err)
	}
	IpamAllocationErrIPUsedOutCounts = allocationErrIPUsedOutCounts

	// spiderpool agent ipam average allocation duration, metric type "float64 gauge"
	err = ipamAllocationAverageDurationSeconds.initGauge(ipamAllocationAverageDurationSecondsName, "spiderpool agent ipam average allocation duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum allocation duration, metric type "float64 gauge"
	err = ipamAllocationMaxDurationSeconds.initGauge(ipamAllocationMaxDurationSecondsName, "spiderpool agent ipam maximum allocation duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	err = ipamAllocationMinDurationSeconds.initGauge(ipamAllocationMinDurationSecondsName, "spiderpool agent ipam minimum allocation duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest allocation duration, metric type "float64 gauge"
	err = ipamAllocationLatestDurationSeconds.initGauge(ipamAllocationLatestDurationSecondsName, "spiderpool agent ipam latest allocation duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam allocation duration bucket, metric type "float64 histogram"
	allocationHistogram, err := newMetricFloat64Histogram(ipamAllocationDurationSecondsName, "histogram of spiderpool agent ipam allocation duration", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationDurationSecondsName, err)
	}
	ipamAllocationDurationSecondsHistogram = allocationHistogram

	// spiderpool agent ipam average allocation limit duration, metric type "float64 gauge"
	err = ipamAllocationAverageLimitDurationSeconds.initGauge(ipamAllocationAverageLimitDurationSecondsName, "spiderpool agent ipam average allocation limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum allocation limit duration, metric type "float64 gauge"
	err = ipamAllocationMaxLimitDurationSeconds.initGauge(ipamAllocationMaxLimitDurationSecondsName, "spiderpool agent ipam maximum allocation limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation limit duration, metric type "float64 gauge"
	err = ipamAllocationMinLimitDurationSeconds.initGauge(ipamAllocationMinLimitDurationSecondsName, "spiderpool agent ipam minimum allocation limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest allocation limit duration, metric type "float64 gauge"
	err = ipamAllocationLatestLimitDurationSeconds.initGauge(ipamAllocationLatestLimitDurationSecondsName, "spiderpool agent ipam latest allocation limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam allocation limit duration bucket, metric type "float64 histogram"
	allocationLimitHistogram, err := newMetricFloat64Histogram(ipamAllocationLimitDurationSecondsName, "histogram of spiderpool agent ipam allocation limit duration", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamAllocationLimitDurationSecondsName, err)
	}
	ipamAllocationLimitDurationSecondsHistogram = allocationLimitHistogram

	return nil
}

// initSpiderpoolAgentReleaseMetrics will init spiderpool-agent IPAM release metrics
func initSpiderpoolAgentReleaseMetrics(ctx context.Context) error {
	// spiderpool agent ipam release total counts, metric type "int64 counter"
	releaseTotalCounts, err := newMetricInt64Counter(ipamReleaseCountsName, "spiderpool agent ipam release total counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamReleaseCountsName, err)
	}
	IpamReleaseTotalCounts = releaseTotalCounts
	IpamReleaseTotalCounts.Add(ctx, 0)

	// spiderpool agent ipam release failure counts, metric type "int64 counter"
	releaseFailureCounts, err := newMetricInt64Counter(ipamReleaseFailureCountsName, "spiderpool agent ipam release failure counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamReleaseFailureCountsName, err)
	}
	IpamReleaseFailureCounts = releaseFailureCounts
	IpamReleaseFailureCounts.Add(ctx, 0)

	// spiderpool agent ipam release update IPPool conflict counts, metric type "int64 counter"
	releaseUpdateIPPoolConflictCounts, err := newMetricInt64Counter(ipamReleaseUpdateIPPoolConflictCountsName, "spiderpool agent ipam release update IPPool conflict counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamReleaseUpdateIPPoolConflictCountsName, err)
	}
	IpamReleaseUpdateIPPoolConflictCounts = releaseUpdateIPPoolConflictCounts

	// spiderpool agent ipam releasing internal error counts, metric type "int64 counter"
	releasingErrInternalCounts, err := newMetricInt64Counter(ipamReleaseErrInternalCountsName, "spiderpool agent ipam release internal error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamReleaseErrInternalCountsName, err)
	}
	IpamReleaseErrInternalCounts = releasingErrInternalCounts

	// spiderpool agent ipam releasing retries exhausted error counts, metric type "int64 counter"
	releasingErrRetriesExhaustedCounts, err := newMetricInt64Counter(ipamReleaseErrRetriesExhaustedCountsName, "spiderpool agent ipam release retries exhausted error counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamReleaseErrRetriesExhaustedCountsName, err)
	}
	IpamReleaseErrRetriesExhaustedCounts = releasingErrRetriesExhaustedCounts

	// spiderpool agent ipam average release duration, metric type "float64 gauge"
	err = ipamReleaseAverageDurationSeconds.initGauge(ipamReleaseAverageDurationSecondsName, "spiderpool agent ipam average release duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum release duration, metric type "float64 gauge"
	err = ipamReleaseMaxDurationSeconds.initGauge(ipamReleaseMaxDurationSecondsName, "spiderpool agent ipam maximum release duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation duration, metric type "float64 gauge"
	err = ipamReleaseMinDurationSeconds.initGauge(ipamReleaseMinDurationSecondsName, "spiderpool agent ipam minimum release duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest release duration, metric type "float64 gauge"
	err = ipamReleaseLatestDurationSeconds.initGauge(ipamReleaseLatestDurationSecondsName, "spiderpool agent ipam latest release duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam allocation duration bucket, metric type "float64 histogram"
	releaseHistogram, err := newMetricFloat64Histogram(ipamReleaseDurationSecondsName, "histogram of spiderpool agent ipam release duration", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamReleaseDurationSecondsName, err)
	}
	ipamReleaseDurationSecondsHistogram = releaseHistogram

	// spiderpool agent ipam average release limit duration, metric type "float64 gauge"
	err = ipamReleaseAverageLimitDurationSeconds.initGauge(ipamReleaseAverageLimitDurationSecondsName, "spiderpool agent ipam average release limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam maximum release limit duration, metric type "float64 gauge"
	err = ipamReleaseMaxLimitDurationSeconds.initGauge(ipamReleaseMaxLimitDurationSecondsName, "spiderpool agent ipam maximum release limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam minimum allocation limit duration, metric type "float64 gauge"
	err = ipamReleaseMinLimitDurationSeconds.initGauge(ipamReleaseMinLimitDurationSecondsName, "spiderpool agent ipam minimum release limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam latest release limit duration, metric type "float64 gauge"
	err = ipamReleaseLatestLimitDurationSeconds.initGauge(ipamReleaseLatestLimitDurationSecondsName, "spiderpool agent ipam latest release limit duration", false)
	if nil != err {
		return err
	}

	// spiderpool agent ipam allocation limit duration bucket, metric type "float64 histogram"
	releaseLimitHistogram, err := newMetricFloat64Histogram(ipamReleaseLimitDurationSecondsName, "histogram of spiderpool agent ipam release limit duration", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamReleaseLimitDurationSecondsName, err)
	}
	ipamReleaseLimitDurationSecondsHistogram = releaseLimitHistogram

	return nil
}

// initSpiderpoolControllerGCMetrics will init spiderpool-controller IP gc metrics
func initSpiderpoolControllerGCMetrics(ctx context.Context) error {
	ipGCTotalCounts, err := newMetricInt64Counter(ipGCCCountsName, "spiderpool controller ip gc total counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %w", ipGCCCountsName, err)
	}
	IPGCTotalCounts = ipGCTotalCounts
	IPGCTotalCounts.Add(ctx, 0)

	ipGCFailureCounts, err := newMetricInt64Counter(ipGCFailureCountsName, "spiderpool controller ip gc total counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %w", ipGCFailureCountsName, err)
	}
	IPGCFailureCounts = ipGCFailureCounts
	ipGCFailureCounts.Add(ctx, 0)

	releaseUpdateIPPoolConflictCounts, err := newMetricInt64Counter(ipamReleaseUpdateIPPoolConflictCountsName, "spiderpool controller gc release update IPPool conflict counts", false)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool agent metric '%s', error: %w", ipamReleaseUpdateIPPoolConflictCountsName, err)
	}
	IpamReleaseUpdateIPPoolConflictCounts = releaseUpdateIPPoolConflictCounts

	return nil
}

func initSpiderpoolControllerCRMetrics(ctx context.Context) error {
	err := TotalSubnetCounts.initGauge(totalSubnetCountsName, "spiderpool total SpiderSubnet counts", false)
	if nil != err {
		return err
	}

	subnetTotalIPCounts, err := newMetricInt64Counter(subnetTotalIPCountsName, "spiderpool single SpiderSubnet corresponding total IP counts", true)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %w", subnetTotalIPCountsName, err)
	}
	SubnetTotalIPCounts = subnetTotalIPCounts

	subnetAvailableIPCounts, err := newMetricInt64Counter(subnetAvailableIPCountsName, "spiderpool single SpiderSubnet corresponding available IP counts", true)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %w", subnetAvailableIPCountsName, err)
	}
	SubnetAvailableIPCounts = subnetAvailableIPCounts

	err = TotalIPPoolCounts.initGauge(totalIPPoolCountsName, "spiderpool total SpiderIPPool counts", false)
	if nil != err {
		return err
	}

	poolTotalIPCounts, err := newMetricInt64Counter(ippoolTotalIPCountsName, "spiderpool single SpiderIPPool corresponding total IP counts", true)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %w", ippoolTotalIPCountsName, err)
	}
	IPPoolTotalIPCounts = poolTotalIPCounts

	poolAvailableIPCounts, err := newMetricInt64Counter(ippoolAvailableIPCountsName, "spiderpool single SpiderIPPool corresponding available IP counts", true)
	if nil != err {
		return fmt.Errorf("failed to new spiderpool controller metric '%s', error: %w", ippoolAvailableIPCountsName, err)
	}
	IPPoolAvailableIPCounts = poolAvailableIPCounts

	err = SubnetPoolCounts.initGauge(subnetIPPoolCountsName, "spider subnet corresponding ippools counts", true)
	if nil != err {
		return err
	}

	return nil
}
