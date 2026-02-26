// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/metric"
)

// initControllerMetricsServer will start an opentelemetry http server for spiderpool controller.
func initControllerMetricsServer(ctx context.Context) {
	metricController, err := metric.InitMetric(ctx, constant.SpiderpoolController,
		controllerContext.Cfg.EnableMetric, controllerContext.Cfg.EnableDebugLevelMetric)
	if nil != err {
		logger.Fatal(err.Error())
	}

	err = metric.InitSpiderpoolControllerMetrics(ctx)
	if nil != err {
		logger.Fatal(err.Error())
	}

	if controllerContext.Cfg.EnableMetric {
		metricsSrv := &http.Server{
			Addr:    fmt.Sprintf(":%s", controllerContext.Cfg.MetricHTTPPort),
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

		controllerContext.MetricsHTTPServer = metricsSrv
	}
}

func monitorMetrics(ctx context.Context) {
	if !controllerContext.Cfg.EnableMetric {
		return
	}

	renewPeriod := time.Duration(controllerContext.Cfg.MetricRenewPeriod) * time.Second
	client := controllerContext.CRDManager.GetClient()

	// record IPPool counts metric
	go wait.UntilWithContext(ctx, func(ctx context.Context) {
		var poolList spiderpoolv2beta1.SpiderIPPoolList
		err := client.List(ctx, &poolList)
		if nil != err {
			logger.Sugar().Errorf("failed to monitor metric TotalIPPoolCounts, error: %w", err)
			return
		}
		metric.TotalIPPoolCounts.Record(int64(len(poolList.Items)))
	}, renewPeriod)

	if controllerContext.Cfg.EnableSpiderSubnet {
		// record Subnet counts metric
		go wait.UntilWithContext(ctx, func(ctx context.Context) {
			var subnetList spiderpoolv2beta1.SpiderSubnetList
			err := client.List(ctx, &subnetList)
			if nil != err {
				logger.Sugar().Errorf("failed to monitor metric TotalSubnetCounts, error: %w", err)
				return
			}
			metric.TotalSubnetCounts.Record(int64(len(subnetList.Items)))
		}, renewPeriod)
	}
}
