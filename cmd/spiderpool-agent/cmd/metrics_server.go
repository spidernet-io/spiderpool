// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"fmt"
	"net/http"

	"k8s.io/client-go/informers"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	"github.com/spidernet-io/spiderpool/pkg/podownercache"
)

// initAgentMetricsServer will start an opentelemetry http server for spiderpool agent.
func initAgentMetricsServer(ctx context.Context) {
	metricController, err := metric.InitMetric(ctx,
		constant.SpiderpoolAgent,
		agentContext.Cfg.EnableMetric,
		agentContext.Cfg.EnableDebugLevelMetric,
	)
	if nil != err {
		logger.Fatal(err.Error())
	}

	var cache podownercache.CacheInterface
	// nolint is used to disable the golint warning for the following line.
	if agentContext.Cfg.EnableRDMAMetric { //nolint:golint
		logger.Info("enable rdma metric exporter")
		informerFactory := informers.NewSharedInformerFactory(agentContext.ClientSet, 0)
		podInformer := informerFactory.Core().V1().Pods().Informer()
		informerFactory.Start(ctx.Done())

		cache, err = podownercache.New(ctx, podInformer, agentContext.CRDManager.GetClient())
		if err != nil {
			logger.Fatal(err.Error())
		}
	} else {
		logger.Info("disable rdma metric exporter")
	}

	err = metric.InitSpiderpoolAgentMetrics(ctx, cache)
	if nil != err {
		logger.Fatal(err.Error())
	}

	if agentContext.Cfg.EnableMetric {
		metricsSrv := &http.Server{
			Addr:    fmt.Sprintf(":%s", agentContext.Cfg.MetricHttpPort),
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

		agentContext.MetricsHttpServer = metricsSrv
	}
}
