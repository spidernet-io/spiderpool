// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics

import (
	"fmt"
	"net/http"
	"time"

	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/sdk/metric/aggregator/histogram"
	controller "go.opentelemetry.io/otel/sdk/metric/controller/basic"
	"go.opentelemetry.io/otel/sdk/metric/export/aggregation"
	processor "go.opentelemetry.io/otel/sdk/metric/processor/basic"
	selector "go.opentelemetry.io/otel/sdk/metric/selector/simple"
)

// meter is a global creator of metric instruments.
var meter metric.Meter

// InitMetricController will set up meter with the input param(required) and create a prometheus exporter.
// returns http handler and error
func InitMetricController(meterName string) (func(w http.ResponseWriter, r *http.Request), error) {
	if len(meterName) == 0 {
		return nil, fmt.Errorf("Failed to init metric controller, meter name is asked to be set!")
	}

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
		return nil, err
	}
	global.SetMeterProvider(exporter.MeterProvider())

	m := global.Meter(meterName)
	meter = m

	return exporter.ServeHTTP, nil
}

// NewMetricInt64Counter will create otel Int64Counter metric. The first param metricName is required and
// the second param is optional.
func NewMetricInt64Counter(metricName string, description string) (metric.Int64Counter, error) {
	if len(metricName) == 0 {
		return metric.Int64Counter{}, fmt.Errorf("Failed to create metric Int64Counter, metric name is asked to be set.")
	}
	return metric.Must(meter).NewInt64Counter(metricName, metric.WithDescription(description)), nil
}

// NewMetricFloat64Histogram will create otel Float64Histogram metric. The first param metricName is required and
// the second param is optional.
func NewMetricFloat64Histogram(metricName string, description string) (metric.Float64Histogram, error) {
	if len(metricName) == 0 {
		return metric.Float64Histogram{}, fmt.Errorf("Failed to create metric Float64Histogram, metric name is asked to be set.")
	}
	return metric.Must(meter).NewFloat64Histogram(metricName, metric.WithDescription(description)), nil
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
