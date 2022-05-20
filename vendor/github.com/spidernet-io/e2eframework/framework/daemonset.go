// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/spidernet-io/e2eframework/tools"
	appsv1 "k8s.io/api/apps/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
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
		return fmt.Errorf("failed to create , a same daemonset %v/%v exists", ds.ObjectMeta.Namespace, ds.ObjectMeta.Name)
	} else {
		t := func() bool {
			existing := &appsv1.DaemonSet{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if !b {
				f.t.Logf("waiting for a same daemonset %v/%v to finish deleting \n", ds.ObjectMeta.Namespace, ds.ObjectMeta.Name)
				return false
			}
			return true
		}
		if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
			return fmt.Errorf("time out to wait a deleting daemonset")
		}
	}
	return f.CreateResource(ds, opts...)
}

func (f *Framework) DeleteDaemonSet(name, nsName string, opts ...client.DeleteOption) error {
	pod := &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: nsName,
			Name:      name,
		},
	}
	return f.DeleteResource(pod, opts...)
}

func (f *Framework) GetDaemonSet(name, namespace string) (*appsv1.DaemonSet, error) {
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

func (f *Framework) WaitDaemonSetReady(name, namespace string, ctx context.Context) (*appsv1.DaemonSet, error) {
	// l := []client.ListOption{
	// 	client.InNamespace(namespace),
	// 	client.MatchingLabels{"app": name},
	// }
	// watchInterface, err := f.KClient.Watch(ctx, &appsv1.DeploymentList{}, l...)
	// l := &client.ListOptions{
	// 	Namespace:     namespace,
	// 	LabelSelector: labels.SelectorFromValidatedSet(metav1.),
	// }

	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.KClient.Watch(ctx, &appsv1.DaemonSetList{}, l)
	if err != nil {
		return nil, fmt.Errorf("failed to Watch: %v", err)
	}
	defer watchInterface.Stop()

	f.t.Logf("pod %v\n", l)
	f.t.Logf("watchInterface %v\n", watchInterface)
	if err != nil {
		return nil, fmt.Errorf("failed to Watch: %v", err)
	}
	defer watchInterface.Stop()
	for {
		select {
		case event, ok := <-watchInterface.ResultChan():
			f.t.Logf("pod %v/%v\n", event, ok)
			if !ok {
				return nil, fmt.Errorf("channel is closed ")
			} else {
				f.t.Logf("daemonset %v/%v %v event \n", namespace, name, event.Type)
				if event.Type == watch.Error {
					return nil, fmt.Errorf("received error event: %+v", event)
				} else if event.Type == watch.Deleted {
					return nil, fmt.Errorf("resource is deleted")
				} else {
					ds, ok := event.Object.(*appsv1.DaemonSet)
					if !ok {
						return nil, fmt.Errorf("failed to get metaObject")
					}
					f.t.Logf("pod %v/%v status=%+v\n", namespace, name)
					//already ready Replicas == all Replicas

					if ds.Status.NumberReady == 0 {
						break

					} else if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled {
						return ds, nil
					}
				}
			}

		case <-ctx.Done():
			return nil, fmt.Errorf("ctx timeout ")
		}
	}

}
