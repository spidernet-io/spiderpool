// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.12.0"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

var (
	// meter is a global creator of metric instruments.
	meter metric.Meter
	// globalEnableMetric determines whether to use metric or not
	globalEnableMetric bool
)

// InitMetricController will set up meter with the input param(required) and create a prometheus exporter.
// returns http handler and error
func InitMetricController(ctx context.Context, meterName string, enableMetric bool) (http.Handler, error) {
	if len(meterName) == 0 {
		return nil, fmt.Errorf("failed to init metric controller, meter name is asked to be set")
	}

	config := prometheus.Config{
		DefaultHistogramBoundaries: []float64{0.1, 0.3, 0.5, 1, 3, 5, 7, 10, 15},
	}

	otelResource, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(constant.SpiderpoolAPIGroup),
		))
	if nil != err {
		return nil, err
	}

	c := controller.New(
		processor.NewFactory(
			selector.NewWithHistogramDistribution(
				histogram.WithExplicitBoundaries(config.DefaultHistogramBoundaries)),
			aggregation.CumulativeTemporalitySelector(),
			processor.WithMemory(true),
		),
		controller.WithResource(otelResource),
	)
	exporter, err := prometheus.New(config, c)
	if nil != err {
		return nil, err
	}
	global.SetMeterProvider(exporter.MeterProvider())

	globalEnableMetric = enableMetric
	if globalEnableMetric {
		meter = global.Meter(meterName)
	} else {
		meter = metric.NewNoopMeterProvider().Meter(meterName)
	}

	return exporter, nil
}

// NewMetricInt64Counter will create otel Int64Counter metric.
// The first param metricName is required and the second param is optional.
func NewMetricInt64Counter(metricName string, description string) (syncint64.Counter, error) {
	if len(metricName) == 0 {
		return nil, fmt.Errorf("failed to create metric Int64Counter, metric name is asked to be set")
	}
	return meter.SyncInt64().Counter(metricName, instrument.WithDescription(description))
}

// NewMetricFloat64Histogram will create otel Float64Histogram metric.
// The first param metricName is required and the second param is optional.
func NewMetricFloat64Histogram(metricName string, description string) (syncfloat64.Histogram, error) {
	if len(metricName) == 0 {
		return nil, fmt.Errorf("failed to create metric Float64Histogram, metric name is asked to be set")
	}
	return meter.SyncFloat64().Histogram(metricName, instrument.WithDescription(description))
}

// NewMetricFloat64Gauge will create otel Float64Gauge metric.
// The first param metricName is required and the second param is optional.
func NewMetricFloat64Gauge(metricName string, description string) (asyncfloat64.Gauge, error) {
	if len(metricName) == 0 {
		return nil, fmt.Errorf("failed to create metric Float64Guage, metric name is asked to be set")
	}

	return meter.AsyncFloat64().Gauge(metricName, instrument.WithDescription(description))
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
