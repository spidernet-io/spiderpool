// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
)

var _ = Describe("check otel metrics with stdout", func() {
	var ctx context.Context
	var cleanup func()
	var err error

	BeforeEach(func() {
		ctx = context.TODO()
		cleanup, err = metrics.SetupMetricsWithStdout(ctx)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		cleanup()
	})

	It("check IP allocation counts metric collection", func() {
		metrics.RecordIncWithAttr(ctx, metrics.NodeAllocatedIpTotalCounts, metrics.TotalIpAttr("node1", "default/pool1"))
		metrics.RecordIncWithAttr(ctx, metrics.NodeDeallocatedIpTotalCounts, metrics.TotalIpAttr("node1", "default/pool1"))
	})

	It("check IP allocation duration metric collection", func() {
		stop := metrics.TimerWithAttr(ctx, metrics.NodeAllocatedIpDuration, metrics.SuccessIpDurationAttr("node1"))
		time.Sleep(time.Second * 2)
		stop()
	})

	It("check IP fail allocation duration attribute", func() {
		failIpDurationAttr := metrics.FailIpDurationAttr("node1")
		Expect(failIpDurationAttr).To(Equal([]attribute.KeyValue{
			metrics.AttrHostName.String("node1"),
			metrics.AttrType.String(metrics.ATTR_TYPE_FAIL),
		}))
	})

})

var _ = Describe("check otel with prometheus", Focus, func() {
	It("should use prometheus as exporter", func() {
		ctx := context.TODO()
		c := make(chan bool)

		err := metrics.SetupMetrics(ctx)
		Expect(err).NotTo(HaveOccurred())
		metrics.RecordIncWithAttr(ctx, metrics.NodeAllocatedIpFailCounts, metrics.FailIpAttr("node2", "default/pool2", "IP allocation overtime"))
		stop := metrics.TimerWithAttr(ctx, metrics.NodeDeallocatedIpDuration, metrics.FailIpDurationAttr("node2"))
		time.Sleep(time.Second * 5)
		stop()

		close(c)
		Consistently(c, "20s").Should(BeClosed())
	})
})
