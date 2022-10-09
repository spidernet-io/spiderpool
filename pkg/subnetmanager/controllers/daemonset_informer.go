// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *controller) StartDaemonSetController(informer cache.SharedIndexInformer, stopper chan struct{}) {
	controllersLogger.Info("Starting DaemonSet informer")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onDaemonSetAdd,
		UpdateFunc: c.onDaemonSetUpdate,
		DeleteFunc: c.onDaemonSetDelete,
	})
	informer.Run(stopper)
}

func (c *controller) onDaemonSetAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetAdd: %v", err)
	}
}

func (c *controller) onDaemonSetUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(context.TODO(), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetUpdate: %v", err)
	}
}

func (c *controller) onDaemonSetDelete(obj interface{}) {
	err := c.cleanupFunc(context.TODO(), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetDelete: %v", err)
	}
}
