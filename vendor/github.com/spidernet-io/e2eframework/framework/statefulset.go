// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"time"

	"k8s.io/utils/pointer"

	"github.com/spidernet-io/e2eframework/tools"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) CreateStatefulSet(sts *appsv1.StatefulSet, opts ...client.CreateOption) error {
	// try to wait for finish last deleting
	fake := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: sts.ObjectMeta.Namespace,
			Name:      sts.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &appsv1.StatefulSet{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return ErrAlreadyExisted
	}
	t := func() bool {
		existing := &appsv1.StatefulSet{}
		e := f.GetResource(key, existing)
		b := api_errors.IsNotFound(e)
		if !b {
			f.Log("waiting for a same statefulSet %v/%v to finish deleting \n", sts.ObjectMeta.Namespace, sts.ObjectMeta.Name)
			return false
		}
		return true
	}
	if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
		return ErrTimeOut
	}

	return f.CreateResource(sts, opts...)
}

func (f *Framework) DeleteStatefulSet(name, namespace string, opts ...client.DeleteOption) error {

	if name == "" || namespace == "" {
		return ErrWrongInput
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(sts, opts...)
}

func (f *Framework) GetStatefulSet(name, namespace string) (*appsv1.StatefulSet, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	key := client.ObjectKeyFromObject(sts)
	existing := &appsv1.StatefulSet{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) GetStatefulSetPodList(sts *appsv1.StatefulSet) (*corev1.PodList, error) {
	if sts == nil {
		return nil, ErrWrongInput
	}

	pods := &corev1.PodList{}
	ops := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(sts.Spec.Selector.MatchLabels),
		},
	}
	e := f.ListResource(pods, ops...)
	if e != nil {
		return nil, e
	}
	return pods, nil
}

func (f *Framework) ScaleStatefulSet(sts *appsv1.StatefulSet, replicas int32) (*appsv1.StatefulSet, error) {
	if sts == nil {
		return nil, ErrWrongInput
	}
	sts.Spec.Replicas = pointer.Int32(replicas)
	err := f.UpdateResource(sts)
	if err != nil {
		return nil, err
	}
	return sts, nil
}

func (f *Framework) WaitStatefulSetReady(name, namespace string, ctx context.Context) (*appsv1.StatefulSet, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.KClient.Watch(ctx, &appsv1.StatefulSetList{}, l)
	if err != nil {
		return nil, ErrWatch
	}
	defer watchInterface.Stop()

	for {
		select {
		// if sts not exist , got no event
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				return nil, ErrChanelClosed
			}
			f.Log("statefulSet %v/%v %v event \n", namespace, name, event.Type)

			// Added    EventType = "ADDED"
			// Modified EventType = "MODIFIED"
			// Deleted  EventType = "DELETED"
			// Bookmark EventType = "BOOKMARK"
			// Error    EventType = "ERROR"
			switch event.Type {
			case watch.Error:
				return nil, ErrEvent
			case watch.Deleted:
				return nil, ErrResDel
			default:
				sts, ok := event.Object.(*appsv1.StatefulSet)
				// metaObject, ok := event.Object.(metav1.Object)
				if !ok {
					return nil, ErrGetObj
				}
				f.Log("statefulSet %v/%v readyReplicas=%+v\n", namespace, name, sts.Status.ReadyReplicas)
				if sts.Status.ReadyReplicas == *(sts.Spec.Replicas) && sts.Status.CurrentReplicas == *(sts.Spec.Replicas) {
					return sts, nil
				}
			}
		case <-ctx.Done():
			return nil, ErrTimeOut
		}
	}
}
