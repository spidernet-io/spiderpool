package metrics_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/metrics"
)

var _ = Describe("Metrics", Focus, func() {
	It("Check otel metrics with stdout", func() {
		ctx := context.TODO()

		cleanup, err := metrics.SetupMetricsWithStdout(ctx)
		Expect(err).NotTo(HaveOccurred())
		defer cleanup()

		metrics.RecordIncWithAttr(ctx, metrics.NodeAllocateIpCounts, metrics.TotalIpAttr("node1", "default/pool1"))
		time.Sleep(time.Second)
		metrics.RecordIncWithAttr(ctx, metrics.NodeDeallocateIpCount, metrics.TotalIpAttr("node1", "default/pool1"))
		time.Sleep(time.Second)
	})
})
