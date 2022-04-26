// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"bufio"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/instrument/syncfloat64"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

const (
	OTEL_COUNTS_SIGNAL = "ippool=\"default_pool\""
)

var (
	verifyCount int

	MetricsTestServerAddr              = ""
	SpiderPoolMeter                    = "spider_pool_meter"
	MetricNodeIPAllocationCountsName   = "Node_IP_Allocation_Counts"
	MetricNodeIPAllocationDurationName = "Node_IP_Allocation_Duration"
)

type IPAllocation struct {
	NodeName string
	PoolName string
	// ....
	MetricNodeIPAllocationCounts   syncint64.Counter
	MetricNodeIPAllocationDuration syncfloat64.Histogram
}

var _ = Describe("metrics", Label("unitest", "metrics_test"), Ordered, func() {
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

		// verifyRecord willverify whether the metrics record correctly
		verifyRecord := func(duration float64) {
			resp, err := http.Get(MetricsTestServerAddr)
			Expect(err).NotTo(HaveOccurred())
			defer func(Body io.ReadCloser) {
				err = Body.Close()
				Expect(err).NotTo(HaveOccurred())
			}(resp.Body)

			reader := bufio.NewReader(resp.Body)

			for {
				line, err := reader.ReadString('\n')
				if nil != err || io.EOF == err {
					if line == "" {
						break
					}
				}

				// verify counts instrument
				if strings.Contains(line, MetricNodeIPAllocationCountsName) && strings.Contains(line, OTEL_COUNTS_SIGNAL) {
					split := strings.Split(line, " ")
					Expect(err).NotTo(HaveOccurred())
					Expect(split[len(split)-1]).Should(Equal("2\n"))
					verifyCount++
				}

				// verify histogram instrument
				if strings.Contains(line, MetricNodeIPAllocationDurationName+"_sum") {
					split := strings.Split(line, " ")
					Expect(err).NotTo(HaveOccurred())
					Expect(split[len(split)-1]).Should(Equal(strconv.FormatFloat(duration, 'f', -1, 64) + "\n"))
					verifyCount++
				}
			}
			Expect(verifyCount).Should(Equal(2))
		}

		// metrics record data
		go func() {
			defer GinkgoRecover()

			// ip allocation logics....
			timeRecorder := metrics.NewTimeRecorder()
			// ....

			// ip allocation succeed
			// record the counter metric without labels.
			ipAllocation.MetricNodeIPAllocationCounts.Add(ctx, 1, attribute.Key("ippool").String("default_pool"))

			time.Sleep(time.Second * 5)
			ipAllocation.MetricNodeIPAllocationCounts.Add(ctx, 1, attribute.Key("ippool").String("default_pool"))

			// record histogram metric with labels.
			duration := timeRecorder.SinceInSeconds()
			ipAllocation.MetricNodeIPAllocationDuration.Record(ctx, duration,
				attribute.Key("hostname").String("node1"),
				attribute.Key("type").String("total"))

			time.Sleep(time.Second * 1)
			verifyRecord(duration)

			close(c)
		}()

		Eventually(c, "20s").Should(BeClosed())
	})

	It("test empty counter metric name", func() {
		_, err := metrics.NewMetricInt64Counter("", "")
		Expect(err).To(HaveOccurred())
	})

	It("test empty histogram metric name", func() {
		_, err := metrics.NewMetricFloat64Histogram("", "")
		Expect(err).To(HaveOccurred())
	})

	It("test InitMetricController with empty meter name", func() {
		_, err := metrics.InitMetricController("")
		Expect(err).To(HaveOccurred())
	})
})
