// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) AddDaemonSetHandler(informer cache.SharedIndexInformer) {
	controllersLogger.Info("Setting up DaemonSet handlers")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onDaemonSetAdd,
		UpdateFunc: c.onDaemonSetUpdate,
		DeleteFunc: c.onDaemonSetDelete,
	})
}

func (c *Controller) onDaemonSetAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetAdd: %v", err)
	}
}

func (c *Controller) onDaemonSetUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(context.TODO(), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetUpdate: %v", err)
	}
}

func (c *Controller) onDaemonSetDelete(obj interface{}) {
	err := c.cleanupFunc(context.TODO(), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetDelete: %v", err)
	}
}
