// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

const (
	ATTR_TYPE_TOTAL   = "total"
	ATTR_TYPE_SUCCESS = "success"
	ATTR_TYPE_FAIL    = "fail"
)

var meter = global.Meter("spiderpool_ipam")

// Define instruments
// The NodeAllocateIpDuration and NodeDeallocateIpDuration unit: second
var (
	NodeAllocatedIpTotalCounts     = metric.Must(meter).NewInt64Counter("superpool_node_allocate_ip_total", metric.WithDescription("The summary counts of this node IP allocations"))
	NodeAllocatedIpSuccessCounts   = metric.Must(meter).NewInt64Counter("superpool_node_allocate_ip_success_total", metric.WithDescription("The summary counts of this node IP success allocations"))
	NodeAllocatedIpFailCounts      = metric.Must(meter).NewInt64Counter("superpool_node_allocate_ip_fail_total", metric.WithDescription("The summary counts of this node IP fail allocations"))
	NodeDeallocatedIpTotalCounts   = metric.Must(meter).NewInt64Counter("superpool_node_deallocate_ip_total", metric.WithDescription("The summary counts of this node IP deallocations"))
	NodeDeallocatedIpSuccessCounts = metric.Must(meter).NewInt64Counter("superpool_node_deallocate_ip_success_total", metric.WithDescription("The summary counts of this node IP success deallocations"))
	NodeDeallocatedIpFailCounts    = metric.Must(meter).NewInt64Counter("superpool_node_deallocate_ip_fail_total", metric.WithDescription("The summary counts of this node IP fail deallocations"))
	NodeAllocatedIpDuration        = metric.Must(meter).NewFloat64Histogram("superpool_node_allocate_ip_duration", metric.WithDescription("The duration of this node IP success allocations"))
	NodeDeallocatedIpDuration      = metric.Must(meter).NewFloat64Histogram("superpool_node_deallocate_ip_duration", metric.WithDescription("The duration of this node IP success deallocations"))
)

// attribute
var (
	AttrHostName = attribute.Key("hostname")
	AttrType     = attribute.Key("type")
	AttrPool     = attribute.Key("pool")
	AttrReason   = attribute.Key("reason")
)

// SinceInSeconds returns the duration of time since the provide time as a float64.
func SinceInSeconds(startTime time.Time) float64 {
	return float64(time.Since(startTime)) / 1e9
}

// RecordIncWithAttr is a function that increments a counter with attributes.
func RecordIncWithAttr(ctx context.Context, m metric.Int64Counter, attr []attribute.KeyValue) {
	meter.RecordBatch(ctx, attr, m.Measurement(1))
}

// TimerWithAttr is a function stopwatch, calling it starts the timer,
// calling the returned function will record the duration.
func TimerWithAttr(ctx context.Context, m metric.Float64Histogram, attr []attribute.KeyValue) func() {
	startTime := time.Now()
	return func() {
		meter.RecordBatch(ctx, attr, m.Measurement(SinceInSeconds(startTime)))
	}
}

// TotalIpAttr returns the attribute for NodeAllocatedIpTotalCounts and NodeDeallocatedIpTotalCounts instrument.
func TotalIpAttr(nodeName, poolName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_TOTAL),
		AttrPool.String(poolName),
	}
}

// SuccessIpAttr returns the attribute for NodeAllocatedIpSuccessCounts and NodeDeallocatedIpSuccessCounts instrument.
func SuccessIpAttr(nodeName, poolName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_SUCCESS),
		AttrPool.String(poolName),
	}
}

// FailIpAttr returns the attribute for NodeAllocatedIpFailCounts and NodeDeallocatedIpFailCounts instrument.
func FailIpAttr(nodeName, poolName, reason string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_FAIL),
		AttrPool.String(poolName),
		AttrReason.String(reason),
	}
}

// SuccessIpDurationAttr returns the attribute for NodeAllocatedIpDuration instrument.
func SuccessIpDurationAttr(nodeName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_SUCCESS),
	}
}

// FailIpDurationAttr returns the attribute for NodeDeallocatedIpDuration instrument.
func FailIpDurationAttr(nodeName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_FAIL),
	}
}

// SetupMetrics will set up an exporter for Prometheus collects.
func SetupMetrics(ctx context.Context) error {
	config := prometheus.Config{
		DefaultHistogramBoundaries: []float64{1, 5, 7, 10, 15, 20, 30},
	}
	c := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries)),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
	)
	exporter, err := prometheus.New(config, c)
	if nil != err {
		return err
	}
	global.SetMeterProvider(exporter.MeterProvider())

	http.HandleFunc("/", exporter.ServeHTTP)
	go func() {
		_ = http.ListenAndServe(":2222", nil)
	}()
	return nil
}

// SetupMetricsWithStdout for development debug with console stdout output.
func SetupMetricsWithStdout(ctx context.Context) (func(), error) {
	exporter, err := stdoutmetric.New(stdoutmetric.WithPrettyPrint())
	if nil != err {
		return nil, fmt.Errorf("create otel stdout exporter: %v", err)
	}
	pusher := controller.New(processor.NewFactory(selector.NewWithInexpensiveDistribution(), exporter), controller.WithExporter(exporter))
	err = pusher.Start(ctx)
	if nil != err {
		return nil, fmt.Errorf("starting push controller: %v", err)
	}
	global.SetMeterProvider(pusher)
	return func() {
		if err := pusher.Stop(ctx); nil != err {
			panic("stopping push controller: " + err.Error())
		}
	}, nil
}

func example() {
	ctx := context.TODO()

	// before real IP allocation, record the Total metric
	RecordIncWithAttr(ctx, NodeAllocatedIpTotalCounts, TotalIpAttr("node1", "default/pool1"))

	// IP allocation success
	RecordIncWithAttr(ctx, NodeAllocatedIpSuccessCounts, SuccessIpAttr("node1", "default/pool1"))

	// IP allocation fail
	err := fmt.Errorf("allocate IP overtime!")
	RecordIncWithAttr(ctx, NodeAllocatedIpFailCounts, FailIpAttr("node1", "default/pool1", err.Error()))

	// record duration
	stop := TimerWithAttr(ctx, NodeAllocatedIpDuration, SuccessIpDurationAttr("node1"))
	defer stop()
}
