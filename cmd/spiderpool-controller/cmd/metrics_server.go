// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net/http"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/metric"
)

// initControllerMetricsServer will start an opentelemetry http server for spiderpool controller.
func initControllerMetricsServer(ctx context.Context) {
	metricController, err := metric.InitMetricController(ctx, constant.SpiderpoolController, controllerContext.Cfg.EnabledMetric)
	if nil != err {
		logger.Fatal(err.Error())
	}

	err = metric.InitSpiderpoolControllerMetrics(ctx)
	if nil != err {
		logger.Fatal(err.Error())
	}

	if controllerContext.Cfg.EnabledMetric {
		metricsSrv := &http.Server{
			Addr:    fmt.Sprintf(":%s", controllerContext.Cfg.MetricHttpPort),
			Handler: metricController,
		}

		go func() {
			if err := metricsSrv.ListenAndServe(); nil != err {
				if err == http.ErrServerClosed {
					return
				}

				logger.Fatal(err.Error())
			}
		}()

		controllerContext.MetricsHttpServer = metricsSrv
	}
}
