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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (f *Framework) CreateDaemonSet(ds *appsv1.DaemonSet, opts ...client.CreateOption) error {
	// try to wait for finish last deleting
	fake := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ds.ObjectMeta.Namespace,
			Name:      ds.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &appsv1.DaemonSet{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create , a same DaemonSet %v/%v exids", ds.ObjectMeta.Namespace, ds.ObjectMeta.Name)
	}
	t := func() bool {
		existing := &appsv1.DaemonSet{}
		e := f.GetResource(key, existing)
		b := api_errors.IsNotFound(e)
		if !b {
			f.t.Logf("waiting for a same DaemonSet %v/%v to finish deleting \n", ds.ObjectMeta.Namespace, ds.ObjectMeta.Name)
			return false
		}
		return true
	}
	if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
		return fmt.Errorf("time out to wait a deleting DaemonSet")
	}

	return f.CreateResource(ds, opts...)
}

func (f *Framework) DeleteDaemonSet(name, namespace string, opts ...client.DeleteOption) error {

	if name == "" || namespace == "" {
		return ErrWrongInput
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(ds, opts...)
}

func (f *Framework) GetDaemonSet(name, namespace string) (*appsv1.DaemonSet, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}
	ds := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	key := client.ObjectKeyFromObject(ds)
	existing := &appsv1.DaemonSet{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) GetDaemonSetPodList(ds *appsv1.DaemonSet) (*corev1.PodList, error) {
	if ds == nil {
		return nil, ErrWrongInput
	}
	pods := &corev1.PodList{}
	ops := []client.ListOption{
		client.MatchingLabelsSelector{
			Selector: labels.SelectorFromSet(ds.Spec.Selector.MatchLabels),
		},
	}
	e := f.ListResource(pods, ops...)
	if e != nil {
		return nil, e
	}
	return pods, nil
}

func (f *Framework) WaitDaemonSetReady(name, namespace string, ctx context.Context) (*appsv1.DaemonSet, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.KClient.Watch(ctx, &appsv1.DaemonSetList{}, l)
	if err != nil {
		return nil, fmt.Errorf("failed to Watch: %v", err)
	}
	defer watchInterface.Stop()

	for {
		select {
		// if ds not exist , got no event
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				return nil, fmt.Errorf("channel is closed ")
			}
			f.t.Logf(" ds %v/%v %v event \n", namespace, name, event.Type)

			// Added    EventType = "ADDED"
			// Modified EventType = "MODIFIED"
			// Deleted  EventType = "DELETED"
			// Bookmark EventType = "BOOKMARK"
			// Error    EventType = "ERROR"
			switch event.Type {
			case watch.Error:
				return nil, fmt.Errorf("received error event: %+v", event)
			case watch.Deleted:
				return nil, fmt.Errorf("resource is deleted")
			default:
				ds, ok := event.Object.(*appsv1.DaemonSet)
				if !ok {
					return nil, fmt.Errorf("failed to get metaObject")
				}

				if ds.Status.NumberReady == 0 {
					break

				} else if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {

					return ds, nil
				}
			}
		case <-ctx.Done():
			return nil, ErrTimeOut
		}
	}
}
