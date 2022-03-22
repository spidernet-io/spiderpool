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

var (
	meter = global.Meter("demo")

	NodeAllocateIpCounts        = metric.Must(meter).NewInt64Counter("superpool_node_allocate_ip_total", metric.WithDescription("The summary counts of this node IP allocations"))
	NodeAllocateIpSuccessCounts = metric.Must(meter).NewInt64Counter("superpool_node_allocate_ip_success_total", metric.WithDescription("The summary counts of this node IP success allocations"))
	NodeAllocateIpFailCounts    = metric.Must(meter).NewInt64Counter("superpool_node_allocate_ip_fail_total", metric.WithDescription("The summary counts of this node IP fail allocations"))

	NodeDeallocateIpCount         = metric.Must(meter).NewInt64Counter("superpool_node_deallocate_ip_total", metric.WithDescription("The summary counts of this node IP deallocations"))
	NodeDeallocateIpSuccessCounts = metric.Must(meter).NewInt64Counter("superpool_node_deallocate_ip_success_total", metric.WithDescription("The summary counts of this node IP success deallocations"))
	NodeDeallocateIpFailCounts    = metric.Must(meter).NewInt64Counter("superpool_node_deallocate_ip_fail_total", metric.WithDescription("The summary counts of this node IP fail deallocations"))

	// allocate latency unit:second
	NodeAllocateIpDuration   = metric.Must(meter).NewFloat64Histogram("superpool_node_allocate_ip_duration", metric.WithDescription("The duration of this node IP success allocations"))
	NodeDeallocateIpDuration = metric.Must(meter).NewFloat64Histogram("superpool_node_deallocate_ip_duration", metric.WithDescription("The duration of this node IP success deallocations"))
)

// attribute
var (
	AttrHostName = attribute.Key("hostname")
	AttrType     = attribute.Key("type")
	AttrPool     = attribute.Key("pool")
	AttrReason   = attribute.Key("reason")
)

func SinceInSeconds(startTime time.Time) float64 {
	return float64(time.Since(startTime)) / 1e9
}

func RecordIncWithAttr(ctx context.Context, m metric.Int64Counter, attr []attribute.KeyValue) {
	meter.RecordBatch(ctx, attr, m.Measurement(1))
}

func TimerWithAttr(ctx context.Context, m metric.Float64Histogram, attr []attribute.KeyValue) func() {
	startTime := time.Now()
	return func() {
		meter.RecordBatch(ctx, attr, m.Measurement(SinceInSeconds(startTime)))
	}
}

func TotalIpAttr(nodeName, poolName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_TOTAL),
		AttrPool.String(poolName),
	}
}

func IPSuccessAttr(nodeName, poolName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_SUCCESS),
		AttrPool.String(poolName),
	}
}

func IPFailAttr(nodeName, poolName, reason string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_FAIL),
		AttrPool.String(poolName),
		AttrReason.String(reason),
	}
}

func IPSuccessDurationAttr(nodeName string) []attribute.KeyValue {
	return []attribute.KeyValue{
		AttrHostName.String(nodeName),
		AttrType.String(ATTR_TYPE_SUCCESS),
	}
}

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
	http.HandleFunc("/metrics", exporter.ServeHTTP)
	go func() {
		_ = http.ListenAndServe(":2222", nil)
	}()
	return nil
}

// for dvelopment
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
	RecordIncWithAttr(ctx, NodeAllocateIpCounts, TotalIpAttr("node1", "default/pool1"))

	// IP allocation success
	RecordIncWithAttr(ctx, NodeAllocateIpSuccessCounts, IPSuccessAttr("node1", "default/pool1"))

	// IP allocation fail
	err := fmt.Errorf("allocate IP overtime!")
	RecordIncWithAttr(ctx, NodeAllocateIpFailCounts, IPFailAttr("node1", "default/pool1", err.Error()))

	// record duration
	stop := TimerWithAttr(ctx, NodeAllocateIpDuration, IPSuccessDurationAttr("node1"))
	defer stop()
}
