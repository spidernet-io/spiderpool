// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) StartDeploymentController(informer cache.SharedIndexInformer, stopper chan struct{}) {
	controllersLogger.Info("Starting Deployment informer")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onDeploymentAdd,
		UpdateFunc: c.onDeploymentUpdate,
		DeleteFunc: c.onDeploymentDelete,
	})
	informer.Run(stopper)
}

func (c *Controller) onDeploymentAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDeploymentAdd: %v", err)
	}
}

func (c *Controller) onDeploymentUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(context.TODO(), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDeploymentUpdate: %v", err)
	}
}

func (c *Controller) onDeploymentDelete(obj interface{}) {
	err := c.cleanupFunc(context.TODO(), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDeploymentDelete: %v", err)
	}
}
