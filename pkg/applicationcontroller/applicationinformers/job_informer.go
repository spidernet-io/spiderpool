// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) AddJobController(informer cache.SharedIndexInformer) error {
	controllersLogger.Info("Setting up Job informer")

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onJobAdd,
		UpdateFunc: c.onJobUpdate,
		DeleteFunc: c.onJobDelete,
	})
	if nil != err {
		return err
	}

	return nil
}

func (c *Controller) onJobAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onJobAdd: %w", err)
	}
}

func (c *Controller) onJobUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onJobUpdate: %w", err)
	}
}

func (c *Controller) onJobDelete(obj interface{}) {
	err := c.cleanupFunc(logutils.IntoContext(context.TODO(), controllersLogger), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onJobDelete: %w", err)
	}
}
