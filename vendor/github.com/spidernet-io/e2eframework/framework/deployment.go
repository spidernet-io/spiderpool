// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"time"

	"github.com/spidernet-io/e2eframework/tools"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) CreateDeployment(dpm *appsv1.Deployment, opts ...client.CreateOption) error {
	// try to wait for finish last deleting
	fake := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: dpm.ObjectMeta.Namespace,
			Name:      dpm.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &appsv1.Deployment{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return ErrAlreadyExisted
	}
	t := func() bool {
		existing := &appsv1.Deployment{}
		e := f.GetResource(key, existing)
		b := api_errors.IsNotFound(e)
		if !b {
			f.Log("waiting for a same deployment %v/%v to finish deleting \n", dpm.ObjectMeta.Namespace, dpm.ObjectMeta.Name)
			return false
		}
		return true
	}
	if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
		return ErrTimeOut
	}
	return f.CreateResource(dpm, opts...)
}

func (f *Framework) DeleteDeployment(name, namespace string, opts ...client.DeleteOption) error {

	if name == "" || namespace == "" {
		return ErrWrongInput
	}

	pod := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(pod, opts...)
}

func (f *Framework) GetDeploymnet(name, namespace string) (*appsv1.Deployment, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	dpm := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	key := client.ObjectKeyFromObject(dpm)
	existing := &appsv1.Deployment{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) GetDeploymentPodList(dpm *appsv1.Deployment) (*corev1.PodList, error) {

	if dpm == nil {
		return nil, ErrWrongInput
	}

	pods := &corev1.PodList{}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(dpm.Spec.Selector.MatchLabels),
		},
	}
	e := f.ListResource(pods, opts...)
	if e != nil {
		return nil, e
	}
	return pods, nil
}

func (f *Framework) ScaleDeployment(dpm *appsv1.Deployment, replicas int32) (*appsv1.Deployment, error) {
	if dpm == nil {
		return nil, ErrWrongInput
	}

	dpm.Spec.Replicas = pointer.Int32(replicas)
	err := f.UpdateResource(dpm)
	if err != nil {
		return nil, err
	}
	return dpm, nil
}

func (f *Framework) WaitDeploymentReady(name, namespace string, ctx context.Context) (*appsv1.Deployment, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.KClient.Watch(ctx, &appsv1.DeploymentList{}, l)
	if err != nil {
		return nil, ErrWatch
	}
	defer watchInterface.Stop()

	for {
		select {
		case event, ok := <-watchInterface.ResultChan():
			f.Log("deployment %v/%v\n", event, ok)
			if !ok {
				return nil, ErrChanelClosed
			}
			f.Log("deployment %v/%v %v event \n", namespace, name, event.Type)
			switch event.Type {
			case watch.Error:
				return nil, ErrEvent
			case watch.Deleted:
				return nil, ErrResDel
			default:
				dpm, ok := event.Object.(*appsv1.Deployment)
				if !ok {
					return nil, ErrGetObj
				}
				f.Log("deployment %v/%v readyReplicas=%+v\n", namespace, name, dpm.Status.ReadyReplicas)
				if dpm.Status.ReadyReplicas == *(dpm.Spec.Replicas) {
					return dpm, nil
				}
			}
		case <-ctx.Done():
			return nil, ErrTimeOut
		}
	}
}
