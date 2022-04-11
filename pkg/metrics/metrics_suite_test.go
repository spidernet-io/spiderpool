// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package metrics_test

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/metrics"
)

var (
	listener   net.Listener
	server     *http.Server
	httpHandle http.Handler
	err        error
	ctx        context.Context
)

func TestMetrics(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Metrics Suite")
}

// start a http server to set up an otel exporter
var _ = BeforeSuite(func() {
	ctx = context.TODO()

	server = new(http.Server)
	httpHandle, err = metrics.InitMetricController(SpiderPoolMeter)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		defer GinkgoRecover()

		/* Obtain a reference to a local port for the given sock,
		 * if snum is zero it means select any available local port.
		 */
		listener, err = net.Listen("tcp", net.JoinHostPort("0.0.0.0", "0"))
		Expect(err).NotTo(HaveOccurred())

		MetricsTestServerAddr = "http://" + listener.Addr().String()

		server.Handler = httpHandle
		server.IdleTimeout = time.Second * 10
		server.ReadTimeout = time.Second * 30
		server.WriteTimeout = time.Second * 60

		err = server.Serve(listener)
		if nil != err && err != http.ErrServerClosed {
			By("Error: Otel metrics http server failed. " + err.Error())
		}

	}()
})

// close http listener
var _ = AfterSuite(func() {
	err = server.Shutdown(ctx)
	Expect(err).NotTo(HaveOccurred())
})
