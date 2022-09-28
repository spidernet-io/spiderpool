// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

func (sm *subnetManager) StartDeploymentController(informer cache.SharedIndexInformer, stopper chan struct{}) {
	logger.Info("Starting Deployment informer")

	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    sm.onDeploymentAdd,
		UpdateFunc: sm.onDeploymentUpdate,
		DeleteFunc: sm.onDeploymentDelete,
	})
	informer.Run(stopper)
}

func (sm *subnetManager) onDeploymentAdd(obj interface{}) {
	ctx := context.TODO()

	deployment := obj.(*appsv1.Deployment)
	deployment = deployment.DeepCopy()

	podSubnetConfig, err := getSubnetConfigFromPodAnno(deployment.Spec.Template.Annotations, getAppReplicas(deployment.Spec.Replicas))
	if nil != err {
		logger.Sugar().Errorf("onDeploymentAdd: failed to get deployment '%s/%s' subnet configuration, error: %v", deployment.Namespace, deployment.Name, err)
		return
	}

	if podSubnetConfig == nil {
		// no subnet manager function annotation, use the default IPAM mode
		return
	}

	logger.Sugar().Debugf("onDeploymentAdd: deployment '%v', SpiderSubnet configuration '%+v'", deployment, *podSubnetConfig)

	// deployment.Kind value is "" in informer
	err = sm.reconcile(ctx, podSubnetConfig, constant.OwnerDeployment, deployment, deployment.Spec.Template.GetLabels(), getAppReplicas(deployment.Spec.Replicas))
	if nil != err {
		logger.Sugar().Errorf("onDeploymentAdd: failed to execute deployment '%s/%s' subnet manager reconcile, error: %v", deployment.Namespace, deployment.Name, err)
		return
	}
}

func (sm *subnetManager) onDeploymentUpdate(oldObj interface{}, newObj interface{}) {
	oldDeployment := oldObj.(*appsv1.Deployment)
	newDeployment := newObj.(*appsv1.Deployment)

	oldSubnetConfig, err := getSubnetConfigFromPodAnno(oldDeployment.Spec.Template.Annotations, getAppReplicas(oldDeployment.Spec.Replicas))
	if nil != err {
		logger.Sugar().Errorf("onDeploymentUpdate: failed get old deployment '%s/%s' subnet configuration, error: %v", oldDeployment.Namespace, oldDeployment.Name, err)
		return
	}

	newSubnetConfig, err := getSubnetConfigFromPodAnno(newDeployment.Spec.Template.Annotations, getAppReplicas(newDeployment.Spec.Replicas))
	if nil != err {
		logger.Sugar().Errorf("onDeploymentUpdate: failed get new deployment '%s/%s' subnet configuration, error: %v", oldDeployment.Namespace, oldDeployment.Name, err)
		return
	}

	ctx := logutils.IntoContext(context.TODO(), logger.With(zap.Any("new deployment", fmt.Sprintf("'%s/%s'", newDeployment.Namespace, newDeployment.Name))))
	if hasSubnetConfigChanged(ctx, oldSubnetConfig, newSubnetConfig) || getAppReplicas(oldDeployment.Spec.Replicas) != getAppReplicas(newDeployment.Spec.Replicas) {
		logger.Sugar().Debugf("onDeploymentUpdate: old deployment '%v' and new deployment '%v' are going to reconcile", oldDeployment, newDeployment)
		newDeployment = newDeployment.DeepCopy()
		err = sm.reconcile(context.TODO(), newSubnetConfig, constant.OwnerDeployment, newDeployment, newDeployment.GetLabels(), getAppReplicas(newDeployment.Spec.Replicas))
		if nil != err {
			logger.Sugar().Errorf("onDeploymentUpdate: failed to execute deployment '%s/%s' subnet manager reconcile, error: %v", newDeployment.Namespace, newDeployment.Name, err)
			return
		}
	}
}

func (sm *subnetManager) onDeploymentDelete(obj interface{}) {
	deployment := obj.(*appsv1.Deployment)

	podSubnetConfig, err := getSubnetConfigFromPodAnno(deployment.Spec.Template.Annotations, getAppReplicas(deployment.Spec.Replicas))
	if nil != err {
		logger.Sugar().Errorf("onDeploymentDelete: failed to get deployment '%s/%s' subnet configuration, error: %v", deployment.Namespace, deployment.Name, err)
		return
	}

	if podSubnetConfig == nil {
		// no subnet manager function annotation, use the default IPAM mode
		return
	}

	if !podSubnetConfig.reclaimIPPool {
		logger.Sugar().Infof("onDeploymentDelete: deployment '%s/%s' configuration reclaim IPPool already set to false, no need to delete corresponding IPPools", deployment.Namespace, deployment.Name)
		return
	}

	ctx := context.TODO()
	deployment = deployment.DeepCopy()

	var v4Pool, v6Pool *spiderpoolv1.SpiderIPPool
	if podSubnetConfig.subnetManagerV4 != "" {
		v4Pool, err = sm.RetrieveIPPool(ctx, constant.OwnerDeployment, deployment, podSubnetConfig.subnetManagerV4, constant.IPv4)
		if nil != err {
			logger.Sugar().Errorf("onDeploymentDelete: failed to retrieve Deployment '%s/%s' V4 IPPool, error: %v", deployment.Namespace, deployment.Name, err)
			return
		}
	}

	if podSubnetConfig.subnetManagerV6 != "" {
		v6Pool, err = sm.RetrieveIPPool(ctx, constant.OwnerDeployment, deployment, podSubnetConfig.subnetManagerV6, constant.IPv6)
		if nil != err {
			logger.Sugar().Errorf("onDeploymentDelete: failed to retrieve Deployment '%s/%s' V6 IPPool, error: %v", deployment.Namespace, deployment.Name, err)
			return
		}
	}

	{
		wg := new(sync.WaitGroup)

		deletePool := func(pool *spiderpoolv1.SpiderIPPool) {
			defer wg.Done()
			err := sm.ipPoolManager.DeleteIPPool(ctx, pool)
			if nil != err {
				logger.Sugar().Errorf("onDeploymentDelete: failed to delete IPPool '%s', error: %v", pool.Name, err)
				return
			}

			logger.Sugar().Infof("onDeploymentDelete: delete IPPool '%s' successfully", pool.Name)
		}

		if v4Pool != nil {
			wg.Add(1)
			go deletePool(v4Pool)
		}

		if v6Pool != nil {
			wg.Add(1)
			go deletePool(v6Pool)
		}

		wg.Wait()
	}
}
