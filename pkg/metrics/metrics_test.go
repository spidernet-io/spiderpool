// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var (
	SpiderPoolMeter                    = "spider_pool_meter"
	MetricNodeIPAllocationCountsName   = "Node_IP_Allocation_Counts"
	MetricNodeIPAllocationDurationName = "Node_IP_Allocation_Duration"
)

type IPAllocation struct {
	NodeName string
	PoolName string
	// ....
	MetricNodeIPAllocationCounts   metric.Int64Counter
	MetricNodeIPAllocationDuration metric.Float64Histogram
}

var _ = Describe("check otel with prometheus", Ordered, func() {
	var httpHandle func(http.ResponseWriter, *http.Request)
	var err error

	BeforeAll(func() {
		httpHandle, err = metrics.InitMetricController(SpiderPoolMeter)
		Expect(err).NotTo(HaveOccurred())

		http.HandleFunc("/metrics", httpHandle)
		go func() {
			_ = http.ListenAndServe(":2222", nil)
		}()
	})

	It("use prometheus as exporter", func() {
		c := make(chan bool)
		ctx := context.Background()

		ipAllocation := IPAllocation{
			NodeName: "node1",
			PoolName: "default/poo1",
		}

		// register metrics
		metricInt64Counter, err := metrics.NewMetricInt64Counter(MetricNodeIPAllocationCountsName, "The total counts of node IP allocations")
		Expect(err).NotTo(HaveOccurred())
		ipAllocation.MetricNodeIPAllocationCounts = metricInt64Counter
		histogram, err := metrics.NewMetricFloat64Histogram(MetricNodeIPAllocationDurationName, "The duration of node IP allocations")
		Expect(err).NotTo(HaveOccurred())
		ipAllocation.MetricNodeIPAllocationDuration = histogram

		// ip allocation logics....
		timeRecorder := metrics.NewTimeRecorder()
		// ....

		// ip allocation succeed
		// record the counter metric without labels.
		ipAllocation.MetricNodeIPAllocationCounts.Add(ctx, 1)

		time.Sleep(time.Second * 10)
		ipAllocation.MetricNodeIPAllocationCounts.Add(ctx, 1)

		// record histogram metric with labels.
		duration := timeRecorder.SinceInSeconds()
		ipAllocation.MetricNodeIPAllocationDuration.Record(ctx, duration,
			attribute.Key("hostname").String("node1"),
			attribute.Key("type").String("total"))

		close(c)
		//Consistently(c, "60s").Should(BeClosed())
		Eventually(c, "20s").Should(BeClosed())
	})
})
