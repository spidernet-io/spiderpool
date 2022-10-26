// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) StartStatefulSetController(informer cache.SharedIndexInformer, stopper chan struct{}) {
	controllersLogger.Info("Starting StatefulSet informer")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onStatefulSetAdd,
		UpdateFunc: c.onStatefulSetUpdate,
		DeleteFunc: c.onStatefulSetDelete,
	})
	informer.Run(stopper)
}

func (c *Controller) onStatefulSetAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onStatefulSetAdd: %v", err)
	}
}

func (c *Controller) onStatefulSetUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(context.TODO(), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onStatefulSetUpdate: %v", err)
	}
}

func (c *Controller) onStatefulSetDelete(obj interface{}) {
	err := c.cleanupFunc(context.TODO(), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onStatefulSetDelete: %v", err)
	}
}
