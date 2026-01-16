// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) AddDaemonSetHandler(informer cache.SharedIndexInformer) error {
	controllersLogger.Info("Setting up DaemonSet handlers")

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onDaemonSetAdd,
		UpdateFunc: c.onDaemonSetUpdate,
		DeleteFunc: c.onDaemonSetDelete,
	})
	if nil != err {
		return err
	}

	return nil
}

func (c *Controller) onDaemonSetAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetAdd: %w", err)
	}
}

func (c *Controller) onDaemonSetUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetUpdate: %w", err)
	}
}

func (c *Controller) onDaemonSetDelete(obj interface{}) {
	err := c.cleanupFunc(logutils.IntoContext(context.TODO(), controllersLogger), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onDaemonSetDelete: %w", err)
	}
}
