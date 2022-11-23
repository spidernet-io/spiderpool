// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) AddCronJobHandler(informer cache.SharedIndexInformer) {
	controllersLogger.Info("Setting up CronJob handlers")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onCronJobAdd,
		UpdateFunc: c.onCronJobUpdate,
		DeleteFunc: c.onCronJobDelete,
	})
}

func (c *Controller) onCronJobAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onCronJobAdd: %v", err)
	}
}

func (c *Controller) onCronJobUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(context.TODO(), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onCronJobUpdate: %v", err)
	}
}

func (c *Controller) onCronJobDelete(obj interface{}) {
	err := c.cleanupFunc(context.TODO(), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onCronJobDelete: %v", err)
	}
}
