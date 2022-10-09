// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *controller) StartReplicaSetController(informer cache.SharedIndexInformer, stopper chan struct{}) {
	controllersLogger.Info("Starting ReplicaSet informer")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onReplicaSetAdd,
		UpdateFunc: c.onReplicaSetUpdate,
		DeleteFunc: c.onReplicaSetDelete,
	})
	informer.Run(stopper)
}

func (c *controller) onReplicaSetAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onReplicaSetAdd: %v", err)
	}
}

func (c *controller) onReplicaSetUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(context.TODO(), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onReplicaSetUpdate: %v", err)
	}
}

func (c *controller) onReplicaSetDelete(obj interface{}) {
	err := c.cleanupFunc(context.TODO(), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onReplicaSetDelete: %v", err)
	}
}
