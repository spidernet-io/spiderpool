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

func (f *Framework) CreateReplicaSet(rs *appsv1.ReplicaSet, opts ...client.CreateOption) error {
	// try to wait for finish last deleting
	fake := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: rs.ObjectMeta.Namespace,
			Name:      rs.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &appsv1.ReplicaSet{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return ErrAlreadyExisted
	}
	t := func() bool {
		existing := &appsv1.ReplicaSet{}
		e := f.GetResource(key, existing)
		b := api_errors.IsNotFound(e)
		if !b {
			f.Log("waiting for a same ReplicaSet %v/%v to finish deleting \n", rs.ObjectMeta.Namespace, rs.ObjectMeta.Name)
			return false
		}
		return true
	}
	if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
		return ErrTimeOut
	}
	return f.CreateResource(rs, opts...)
}

func (f *Framework) DeleteReplicaSet(name, namespace string, opts ...client.DeleteOption) error {

	if name == "" || namespace == "" {
		return ErrWrongInput
	}

	pod := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(pod, opts...)
}

func (f *Framework) GetReplicaSet(name, namespace string) (*appsv1.ReplicaSet, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	key := client.ObjectKeyFromObject(rs)
	existing := &appsv1.ReplicaSet{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) GetReplicaSetPodList(rs *appsv1.ReplicaSet) (*corev1.PodList, error) {

	if rs == nil {
		return nil, ErrWrongInput
	}

	pods := &corev1.PodList{}
	opts := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(rs.Spec.Selector.MatchLabels),
		},
	}
	e := f.ListResource(pods, opts...)
	if e != nil {
		return nil, e
	}
	return pods, nil
}

func (f *Framework) ScaleReplicaSet(rs *appsv1.ReplicaSet, replicas int32) (*appsv1.ReplicaSet, error) {
	if rs == nil {
		return nil, ErrWrongInput
	}
	rs.Spec.Replicas = pointer.Int32(replicas)
	err := f.UpdateResource(rs)
	if err != nil {
		return nil, err
	}
	return rs, nil
}

func (f *Framework) WaitReplicaSetReady(name, namespace string, ctx context.Context) (*appsv1.ReplicaSet, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.KClient.Watch(ctx, &appsv1.ReplicaSetList{}, l)
	if err != nil {
		return nil, ErrWatch
	}
	defer watchInterface.Stop()

	for {
		select {
		case event, ok := <-watchInterface.ResultChan():
			f.Log("ReplicaSet %v/%v\n", event, ok)
			if !ok {
				return nil, ErrChanelClosed
			}
			f.Log("ReplicaSet %v/%v %v event \n", namespace, name, event.Type)
			switch event.Type {
			case watch.Error:
				return nil, ErrEvent
			case watch.Deleted:
				return nil, ErrResDel
			default:
				rs, ok := event.Object.(*appsv1.ReplicaSet)
				if !ok {
					return nil, ErrGetObj
				}
				f.Log("ReplicaSet %v/%v readyReplicas=%+v\n", namespace, name, rs.Status.ReadyReplicas)
				if rs.Status.ReadyReplicas == *(rs.Spec.Replicas) {
					return rs, nil
				}
			}
		case <-ctx.Done():
			return nil, ErrTimeOut
		}
	}
}
