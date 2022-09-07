// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/cache"
)

func (sm *subnetMgr) StartDeploymentController(informer cache.SharedIndexInformer) {
	logger.Info("Starting Deployment informer")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sm.onDeploymentAdd,
		UpdateFunc: sm.onDeploymentUpdate,
		DeleteFunc: nil,
	})
	informer.Run(sm.stop)
}

func (sm *subnetMgr) onDeploymentAdd(obj interface{}) {
	if !sm.leader.IsElected() {
		return
	}

	ctx := context.TODO()

	deployment := obj.(*appsv1.Deployment)
	deployment = deployment.DeepCopy()

	podSubnetConfig, err := getObjSubnetConfig(deployment.Spec.Template.Annotations)
	if nil != err {
		logger.Sugar().Errorf("onDeploymentAdd: failed to get deployment '%s/%s' subnet configuration, error: %v", deployment.Namespace, deployment.Name, err)
		return
	}

	err = sm.reconcile(ctx, podSubnetConfig, deployment.Kind, deployment)
	if nil != err {
		logger.Sugar().Errorf("onDeploymentAdd: failed to execute deployment '%s/%s' subnet manager reconcile, error: %v", deployment.Namespace, deployment.Name, err)
		return
	}
}

func (sm *subnetMgr) onDeploymentUpdate(oldObj interface{}, newObj interface{}) {
	if !sm.leader.IsElected() {
		return
	}

	oldDeployment := oldObj.(*appsv1.Deployment)
	newDeployment := newObj.(*appsv1.Deployment)

	oldSubnetConfig, err := getObjSubnetConfig(oldDeployment.Labels)
	if nil != err {
		logger.Sugar().Errorf("onDeploymentUpdate: failed get old deployment '%s/%s' subnet configuration, error: %v", oldDeployment.Namespace, oldDeployment.Name, err)
		return
	}

	newSubnetConfig, err := getObjSubnetConfig(newDeployment.Labels)
	if nil != err {
		logger.Sugar().Errorf("onDeploymentUpdate: failed get new deployment '%s/%s' subnet configuration, error: %v", oldDeployment.Namespace, oldDeployment.Name, err)
		return
	}

	if oldSubnetConfig == newSubnetConfig {
		return
	}

	newDeployment = newDeployment.DeepCopy()
	err = sm.reconcile(context.TODO(), newSubnetConfig, newDeployment.Kind, newDeployment)
	if nil != err {
		logger.Sugar().Errorf("onDeploymentUpdate: failed to execute deployment '%s/%s' subnet manager reconcile, error: %v", newDeployment.Namespace, newDeployment.Name, err)
		return
	}
}
