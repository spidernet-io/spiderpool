// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metric

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"go.opentelemetry.io/otel/exporters/prometheus"
	api "go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/sdk"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
)

const debugMetrics = "debug-metrics"

var (
	// meter is a global creator of metric instruments.
	meter, debugLevelMeter api.Meter
	// globalEnableMetric determines whether to use metric or not
	globalEnableMetric bool
)

// InitMetric will set up meter with the input param(required) and create a prometheus exporter.
func InitMetric(ctx context.Context, meterName string, enableMetric, enableDebugLevelMetrics bool) (http.Handler, error) {
	if len(meterName) == 0 {
		return nil, fmt.Errorf("failed to init metric controller, meter name is asked to be set")
	}

	globalEnableMetric = enableMetric
	otelResource, err := resource.New(ctx,
		resource.WithTelemetrySDK(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(constant.SpiderpoolAPIGroup),
		))
	if nil != err {
		return nil, err
	}

	exporter, err := prometheus.New()
	if nil != err {
		return nil, err
	}
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(exporter),
		sdkmetric.WithResource(otelResource),
		sdkmetric.WithView(sdkmetric.NewView(
			sdkmetric.Instrument{
				Name: fmt.Sprintf(metricPrefix + "*"),
				Kind: sdkmetric.InstrumentKindHistogram,
			},
			sdkmetric.Stream{Aggregation: sdkmetric.AggregationExplicitBucketHistogram{
				Boundaries: []float64{0.1, 0.3, 0.5, 1, 3, 5, 7, 10, 15},
			}},
		)),
	)

	debugMeterName := fmt.Sprintf("%s-%s", meterName, debugMetrics)
	meter = noop.NewMeterProvider().Meter("")
	debugLevelMeter = noop.NewMeterProvider().Meter("")
	if globalEnableMetric {
		meter = provider.Meter(meterName, api.WithInstrumentationVersion(sdk.Version()))
		if enableDebugLevelMetrics {
			debugLevelMeter = provider.Meter(debugMeterName, api.WithInstrumentationVersion(sdk.Version()))
		}
	}

	return promhttp.Handler(), nil
}

// newMetricInt64Counter will create otel Int64Counter metric.
// The first param metricName is required and the second param is optional.
func newMetricInt64Counter(metricName string, description string, isDebugLevel bool) (api.Int64Counter, error) {
	if len(metricName) == 0 {
		return nil, fmt.Errorf("failed to create metric Int64Counter, metric name is asked to be set")
	}

	m := meter
	if isDebugLevel {
		m = debugLevelMeter
	}
	return m.Int64Counter(metricName, api.WithDescription(description))
}

// newMetricFloat64Histogram will create otel Float64Histogram metric.
// The first param metricName is required and the second param is optional.
// Notice: if you want to match the quantile {0.1, 0.3, 0.5, 1, 3, 5, 7, 10, 15}, please let the metric name match regex "*_histogram",
// otherwise it will match the  otel default quantile.
func newMetricFloat64Histogram(metricName string, description string, isDebugLevel bool) (api.Float64Histogram, error) {
	if len(metricName) == 0 {
		return nil, fmt.Errorf("failed to create metric Float64Histogram, metric name is asked to be set")
	}

	m := meter
	if isDebugLevel {
		m = debugLevelMeter
	}
	return m.Float64Histogram(metricName, api.WithDescription(description))
}

// newMetricFloat64Gauge will create otel Float64Gauge metric.
// The first param metricName is required and the second param is optional.
func newMetricFloat64Gauge(metricName string, description string, isDebugLevel bool) (api.Float64ObservableGauge, error) {
	if len(metricName) == 0 {
		return nil, fmt.Errorf("failed to create metric Float64Guage, metric name is asked to be set")
	}

	m := meter
	if isDebugLevel {
		m = debugLevelMeter
	}
	return m.Float64ObservableGauge(metricName, api.WithDescription(description))
}

// newMetricInt64Gauge will create otel Int64Gauge metric.
// The first param metricName is required and the second param is optional.
func newMetricInt64Gauge(metricName string, description string, isDebugLevel bool) (api.Int64ObservableGauge, error) {
	if len(metricName) == 0 {
		return nil, fmt.Errorf("failed to create metric Float64Guage, metric name is asked to be set")
	}

	m := meter
	if isDebugLevel {
		m = debugLevelMeter
	}
	return m.Int64ObservableGauge(metricName, api.WithDescription(description))
}

var _ TimeRecorder = &timeRecorder{}

// timeRecorder owns a field to record start time.
type timeRecorder struct {
	startTime time.Time
}

// TimeRecorder will help you to compute time duration.
type TimeRecorder interface {
	SinceInSeconds() float64
}

// NewTimeRecorder will create TimeRecorder and record the current time.
func NewTimeRecorder() TimeRecorder {
	t := timeRecorder{}
	t.startTime = time.Now()
	return &t
}

// SinceInSeconds returns the duration of time since the start time as a float64.
func (t *timeRecorder) SinceInSeconds() float64 {
	return float64(time.Since(t.startTime)) / 1e9
}
