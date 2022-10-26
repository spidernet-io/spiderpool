// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) StartJobController(informer cache.SharedIndexInformer, stopper chan struct{}) {
	controllersLogger.Info("Starting Job informer")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onJobAdd,
		UpdateFunc: c.onJobUpdate,
		DeleteFunc: c.onJobDelete,
	})
	informer.Run(stopper)
}

func (c *Controller) onJobAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onJobAdd: %v", err)
	}
}

func (c *Controller) onJobUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(context.TODO(), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onJobUpdate: %v", err)
	}
}

func (c *Controller) onJobDelete(obj interface{}) {
	err := c.cleanupFunc(context.TODO(), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onJobDelete: %v", err)
	}
}
