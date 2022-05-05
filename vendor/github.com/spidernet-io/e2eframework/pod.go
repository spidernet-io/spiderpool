// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package e2eframework

import (
	"context"
	"fmt"
	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func (f *Framework) CreatePod(pod *corev1.Pod, opts ...client.CreateOption) error {
	// try to wait for finish last deleting
	fake := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: pod.ObjectMeta.Namespace,
			Name:      pod.ObjectMeta.Name,
		},
	}
	key := client.ObjectKeyFromObject(fake)
	existing := &corev1.Pod{}
	e := f.GetResource(key, existing)
	if e == nil && existing.ObjectMeta.DeletionTimestamp == nil {
		return fmt.Errorf("failed to create , a same pod %v/%v exists", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
	} else {
		t := func() bool {
			existing := &corev1.Pod{}
			e := f.GetResource(key, existing)
			b := api_errors.IsNotFound(e)
			if !b {
				f.t.Logf("waiting for a same pod %v/%v to finish deleting \n", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
				return false
			}
			return true
		}
		if tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) == false {
			return fmt.Errorf("time out to wait a deleting pod")
		}
	}

	return f.CreateResource(pod, opts...)
}

func (f *Framework) DeletePod(name, namespace string, opts ...client.DeleteOption) error {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(pod, opts...)
}

func (f *Framework) GetPod(name, namespace string) (*corev1.Pod, error) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	key := client.ObjectKeyFromObject(pod)
	existing := &corev1.Pod{}
	e := f.GetResource(key, existing)
	if e != nil {
		return nil, e
	}
	return existing, e
}

func (f *Framework) WaitPodStarted(name, namespace string, ctx context.Context) (*corev1.Pod, error) {

	// refer to https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/client/watch_test.go
	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.KClient.Watch(ctx, &corev1.PodList{}, l)
	if err != nil {
		return nil, fmt.Errorf("failed to Watch: %v", err)
	}
	defer watchInterface.Stop()

	for {
		select {
		// if pod not exist , got no event
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				return nil, fmt.Errorf("channel is closed ")
			} else {
				f.t.Logf("pod %v/%v %v event \n", namespace, name, event.Type)

				// Added    EventType = "ADDED"
				// Modified EventType = "MODIFIED"
				// Deleted  EventType = "DELETED"
				// Bookmark EventType = "BOOKMARK"
				// Error    EventType = "ERROR"
				if event.Type == watch.Error {
					return nil, fmt.Errorf("received error event: %+v", event)
				} else if event.Type == watch.Deleted {
					return nil, fmt.Errorf("resource is deleted")
				} else {
					pod, ok := event.Object.(*corev1.Pod)
					// metaObject, ok := event.Object.(metav1.Object)
					if !ok {
						return nil, fmt.Errorf("failed to get metaObject")
					}
					f.t.Logf("pod %v/%v status=%+v\n", namespace, name, pod.Status.Phase)
					if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
						break
					} else {
						return pod, nil
					}
				}
			}
		case <-ctx.Done():
			return nil, fmt.Errorf("ctx timeout ")
		}
	}
}
