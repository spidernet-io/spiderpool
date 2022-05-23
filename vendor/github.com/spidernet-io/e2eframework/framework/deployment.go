// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"fmt"
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
		return fmt.Errorf("failed to create , a same deployment %v/%v exists", dpm.ObjectMeta.Namespace, dpm.ObjectMeta.Name)
	}
	t := func() bool {
		existing := &appsv1.Deployment{}
		e := f.GetResource(key, existing)
		b := api_errors.IsNotFound(e)
		if !b {
			f.t.Logf("waiting for a same deployment %v/%v to finish deleting \n", dpm.ObjectMeta.Namespace, dpm.ObjectMeta.Name)
			return false
		}
		return true
	}
	if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
		return fmt.Errorf("time out to wait a deleting deployment")
	}
	return f.CreateResource(dpm, opts...)
}

func (f *Framework) DeleteDeployment(name, namespace string, opts ...client.DeleteOption) error {
	// switch {
	// case name == "":
	// 	return fmt.Errorf("the deployment name %v not to be empty", name)
	// case namespace == "":
	// 	return fmt.Errorf("the deployment namespace %v not to be empty", namespace)
	// }

	if name == "" {
		return fmt.Errorf("the deployment name %v not to be empty", name)
	} else if namespace == "" {
		return fmt.Errorf("the deployment namespace %v not to be empty", namespace)
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

	if name == "" {
		return nil, fmt.Errorf("the deployment name %v not to be empty", name)
	} else if namespace == "" {
		return nil, fmt.Errorf("the deployment namespace %v not to be empty", namespace)
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
		return nil, fmt.Errorf("dpm cannot be nil")
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
		return nil, fmt.Errorf("deployment cannot be nil")
	}
	dpm.Spec.Replicas = pointer.Int32(replicas)
	err := f.UpdateResource(dpm)
	if err != nil {
		return nil, err
	}
	return dpm, nil
}

func (f *Framework) WaitDeploymentReady(name, namespace string, ctx context.Context) (*appsv1.Deployment, error) {

	if name == "" {
		return nil, fmt.Errorf("the deployment name %v not to be empty", name)
	} else if namespace == "" {
		return nil, fmt.Errorf("the deployment namespace %v not to be empty", namespace)
	}

	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.KClient.Watch(ctx, &appsv1.DeploymentList{}, l)
	if err != nil {
		return nil, fmt.Errorf("failed to Watch: %v", err)
	}
	defer watchInterface.Stop()

	for {
		select {
		case event, ok := <-watchInterface.ResultChan():
			f.t.Logf("deployment %v/%v\n", event, ok)
			if !ok {
				return nil, fmt.Errorf("channel is closed ")
			}
			f.t.Logf("deployment %v/%v %v event \n", namespace, name, event.Type)
			switch event.Type {
			case watch.Error:
				return nil, fmt.Errorf("received error event: %+v", event)
			case watch.Deleted:
				return nil, fmt.Errorf("resource is deleted")
			default:
				dpm, ok := event.Object.(*appsv1.Deployment)
				if !ok {
					return nil, fmt.Errorf("failed to get metaObject")
				}
				f.t.Logf("deployment %v/%v readyReplicas=%+v\n", namespace, name, dpm.Status.ReadyReplicas)
				if dpm.Status.ReadyReplicas == *(dpm.Spec.Replicas) {
					return dpm, nil
				}
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("ctx timeout ")
		}
	}
}
