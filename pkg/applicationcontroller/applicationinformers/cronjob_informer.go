// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"context"

	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (c *Controller) AddCronJobHandler(informer cache.SharedIndexInformer) error {
	controllersLogger.Info("Setting up CronJob handlers")

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onCronJobAdd,
		UpdateFunc: c.onCronJobUpdate,
		DeleteFunc: c.onCronJobDelete,
	})
	if nil != err {
		return err
	}

	return nil
}

func (c *Controller) onCronJobAdd(obj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), nil, obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onCronJobAdd: %w", err)
	}
}

func (c *Controller) onCronJobUpdate(oldObj interface{}, newObj interface{}) {
	err := c.reconcileFunc(logutils.IntoContext(context.TODO(), controllersLogger), oldObj, newObj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onCronJobUpdate: %w", err)
	}
}

func (c *Controller) onCronJobDelete(obj interface{}) {
	err := c.cleanupFunc(logutils.IntoContext(context.TODO(), controllersLogger), obj)
	if nil != err {
		controllersLogger.Sugar().Errorf("onCronJobDelete: %w", err)
	}
}
