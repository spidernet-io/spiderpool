// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package draController

import (
	"context"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	clientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/dynamic-resource-allocation/controller"
)

func StartController(ctx context.Context,
	leaderRetryElectGap time.Duration,
	spiderClientset clientset.Interface,
	kubeClient kubernetes.Interface,
	informerFactory informers.SharedInformerFactory) error {

	driver := NewDriver(spiderClientset)
	controller := controller.New(ctx, constant.DRADriverName, driver, kubeClient, informerFactory)

	innerCtx, innerCancel := context.WithCancel(ctx)
	defer innerCancel()
	informerFactory.Start(innerCtx.Done())
	controller.Run(1)

	return nil
}
