// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) AddReplicaSetHandler(informer cache.SharedIndexInformer) error {
	controllersLogger.Info("Setting up ReplicaSet handlers")

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onReplicaSetAdd,
		UpdateFunc: c.onReplicaSetUpdate,
		DeleteFunc: c.onReplicaSetDelete,
	})
	if nil != err {
		return err
	}

	return nil
}

func (c *Controller) onReplicaSetAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onReplicaSetAdd: %w", err)
	}
}

func (c *Controller) onReplicaSetUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onReplicaSetUpdate: %w", err)
	}
}

func (c *Controller) onReplicaSetDelete(obj interface{}) {
	err := c.cleanupFunc(logutils.IntoContext(context.TODO(), controllersLogger), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onReplicaSetDelete: %w", err)
	}
}
