// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) AddDeploymentHandler(informer cache.SharedIndexInformer) error {
	controllersLogger.Info("Setting up Deployment handlers")

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onDeploymentAdd,
		UpdateFunc: c.onDeploymentUpdate,
		DeleteFunc: c.onDeploymentDelete,
	})
	if nil != err {
		return err
	}

	return nil
}

func (c *Controller) onDeploymentAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDeploymentAdd: %w", err)
	}
}

func (c *Controller) onDeploymentUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDeploymentUpdate: %w", err)
	}
}

func (c *Controller) onDeploymentDelete(obj interface{}) {
	err := c.cleanupFunc(logutils.IntoContext(context.TODO(), controllersLogger), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDeploymentDelete: %w", err)
	}
}
