// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) AddStatefulSetHandler(informer cache.SharedIndexInformer) error {
	controllersLogger.Info("Setting up StatefulSet handlers")

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onStatefulSetAdd,
		UpdateFunc: c.onStatefulSetUpdate,
		DeleteFunc: c.onStatefulSetDelete,
	})
	if nil != err {
		return err
	}

	return nil
}

func (c *Controller) onStatefulSetAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onStatefulSetAdd: %w", err)
	}
}

func (c *Controller) onStatefulSetUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onStatefulSetUpdate: %w", err)
	}
}

func (c *Controller) onStatefulSetDelete(obj interface{}) {
	err := c.cleanupFunc(logutils.IntoContext(context.TODO(), controllersLogger), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onStatefulSetDelete: %w", err)
	}
}
