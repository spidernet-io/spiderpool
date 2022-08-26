// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"fmt"
	"time"

	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
				f.Log("waiting for a same pod %v/%v to finish deleting \n", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
				return false
			}
			return true
		}
		if !tools.Eventually(t, f.Config.ResourceDeleteTimeout, time.Second) {
			return ErrTimeOut
		}
	}
	return f.CreateResource(pod, opts...)
}

func (f *Framework) DeletePod(name, namespace string, opts ...client.DeleteOption) error {

	if name == "" || namespace == "" {
		return ErrWrongInput
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
	}
	return f.DeleteResource(pod, opts...)
}

func (f *Framework) GetPod(name, namespace string) (*corev1.Pod, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

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

func (f *Framework) GetPodList(opts ...client.ListOption) (*corev1.PodList, error) {
	pods := &corev1.PodList{}
	e := f.ListResource(pods, opts...)
	if e != nil {
		return nil, e
	}
	return pods, nil
}

func (f *Framework) WaitPodStarted(name, namespace string, ctx context.Context) (*corev1.Pod, error) {

	if name == "" || namespace == "" {
		return nil, ErrWrongInput
	}

	// refer to https://github.com/kubernetes-sigs/controller-runtime/blob/master/pkg/client/watch_test.go
	l := &client.ListOptions{
		Namespace:     namespace,
		FieldSelector: fields.OneTermEqualSelector("metadata.name", name),
	}
	watchInterface, err := f.KClient.Watch(ctx, &corev1.PodList{}, l)
	if err != nil {
		return nil, ErrWatch
	}
	defer watchInterface.Stop()

	for {
		select {
		// if pod not exist , got no event
		case event, ok := <-watchInterface.ResultChan():
			if !ok {
				return nil, ErrChanelClosed
			}
			f.Log("pod %v/%v %v event \n", namespace, name, event.Type)
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
				pod, ok := event.Object.(*corev1.Pod)
				// metaObject, ok := event.Object.(metav1.Object)
				if !ok {
					return nil, fmt.Errorf("failed to get metaObject")
				}
				f.Log("pod %v/%v status=%+v\n", namespace, name, pod.Status.Phase)
				if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
					break
				} else {
					return pod, nil
				}
			}
		case <-ctx.Done():
			return nil, ErrTimeOut
		}
	}
}

func (f *Framework) WaitPodListDeleted(namespace string, label map[string]string, ctx context.Context) error {
	// Query all pods corresponding to the label
	// Delete the resource until the query is empty

	if namespace == "" || label == nil {
		return ErrWrongInput
	}

	opts := []client.ListOption{
		client.InNamespace(namespace),
		client.MatchingLabels(label),
	}
	for {
		select {
		case <-ctx.Done():
			return ErrTimeOut
		default:
			podlist, err := f.GetPodList(opts...)
			if err != nil {
				return err
			} else if len(podlist.Items) == 0 {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}

func (f *Framework) DeletePodUntilFinish(name, namespace string, ctx context.Context, opts ...client.DeleteOption) error {
	// Query all pods by name in namespace
	// Delete the resource until the query is empty
	if namespace == "" || name == "" {
		return ErrWrongInput
	}
	err := f.DeletePod(name, namespace, opts...)
	if err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ErrTimeOut
		default:
			pod, _ := f.GetPod(name, namespace)
			if pod == nil {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}

func (f *Framework) CheckPodListIpReady(podList *corev1.PodList) error {
	var v4IpList = make(map[string]string)
	var v6IpList = make(map[string]string)

	for _, pod := range podList.Items {
		if pod.Status.PodIPs == nil {
			return fmt.Errorf("pod %v failed to assign ip", pod.Name)
		}
		f.Log("pod %v ips: %+v \n", pod.Name, pod.Status.PodIPs)
		if f.Info.IpV4Enabled {
			ip, ok := tools.CheckPodIpv4IPReady(&pod)
			if !ok {
				return fmt.Errorf("pod %v failed to get ipv4 ip", pod.Name)
			}
			if d, ok := v4IpList[ip]; ok {
				return fmt.Errorf("pod %v and %v have conflicted ipv4 ip %v", pod.Name, d, ip)
			}
			v4IpList[ip] = pod.Name
			f.Log("succeeded to check pod %v ipv4 ip \n", pod.Name)
		}
		if f.Info.IpV6Enabled {
			ip, err := tools.CheckPodIpv6IPReady(&pod)
			if !err {
				return fmt.Errorf("pod %v failed to get ipv6 ip", pod.Name)
			}
			if d, ok := v6IpList[ip]; ok {
				return fmt.Errorf("pod %v and %v have conflicted ipv6 ip %v", pod.Name, d, ip)
			}
			v6IpList[ip] = pod.Name
			f.Log("succeeded to check pod %v ipv6 ip \n", pod.Name)
		}
	}
	return nil
}

func (f *Framework) GetPodListByLabel(label map[string]string) (*corev1.PodList, error) {
	if label == nil {
		return nil, ErrWrongInput
	}
	ops := []client.ListOption{
		client.MatchingLabels(label),
	}
	return f.GetPodList(ops...)
}

func (f *Framework) CheckPodListRunning(podList *corev1.PodList) bool {
	if podList == nil {
		return false
	}
	for _, item := range podList.Items {
		if item.Status.Phase != "Running" {
			return false
		}
	}
	return true
}

func (f *Framework) DeletePodList(podList *corev1.PodList, opts ...client.DeleteOption) error {
	if podList == nil {
		return ErrWrongInput
	}
	for _, item := range podList.Items {
		err := f.DeletePod(item.Name, item.Namespace, opts...)
		if err != nil {
			return err
		}
	}
	return nil
}

func (f *Framework) WaitPodListRunning(label map[string]string, expectedPodNum int, ctx context.Context) error {
	if label == nil || expectedPodNum == 0 {
		return ErrWrongInput
	}
	for {
		select {
		default:
			// get pod list
			podList, err := f.GetPodListByLabel(label)
			if err != nil {
				return err
			}
			if len(podList.Items) != expectedPodNum {
				break
			}

			// wait pod list Running
			if f.CheckPodListRunning(podList) {
				return nil
			}
			time.Sleep(time.Second)
		case <-ctx.Done():
			return fmt.Errorf("time out to wait podList ready")
		}
	}
}

func (f *Framework) DeletePodListRepeatedly(label map[string]string, interval time.Duration, ctx context.Context, opts ...client.DeleteOption) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
			podList, e1 := f.GetPodListByLabel(label)
			if e1 != nil {
				return e1
			}
			e2 := f.DeletePodList(podList, opts...)
			if e2 != nil {
				return e2
			}
			time.Sleep(interval)
		}
	}
}

func (f *Framework) DeletePodListUntilReady(podList *corev1.PodList, timeOut time.Duration, opts ...client.DeleteOption) (*corev1.PodList, error) {
	if podList == nil {
		return nil, ErrWrongInput
	}

	err := f.DeletePodList(podList, opts...)
	if err != nil {
		f.Log("failed to DeletePodList")
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()
OUTER:
	for {
		time.Sleep(time.Second)
		select {
		case <-ctx.Done():
			return nil, ErrTimeOut
		default:
		}
		f.Log("checking restarted pod ")

		podListWithLabel, err := f.GetPodListByLabel(podList.Items[0].Labels)
		if err != nil {
			f.Log("failed to GetPodListByLabel , reason=%v", err)
			continue
		}

		if len(podListWithLabel.Items) != len(podList.Items) {
			continue
		}

		for _, newPod := range podListWithLabel.Items {
			if newPod.Status.Phase != corev1.PodRunning || newPod.DeletionTimestamp != nil {
				continue OUTER
			}
			for _, oldPod := range podList.Items {
				if newPod.ObjectMeta.UID == oldPod.ObjectMeta.UID {
					continue OUTER
				}
			}

			// make sure pod ready
			for _, newPodContainer := range newPod.Status.ContainerStatuses {
				if !newPodContainer.Ready {
					continue OUTER
				}
			}
		}
		return podListWithLabel, nil
	}
}

// Waiting for all pods in all namespaces to run
func (f *Framework) WaitAllPodUntilRunning(ctx context.Context) error {
	var AllPodList *corev1.PodList
	var err error

	for {
		select {
		case <-ctx.Done():
			return ErrTimeOut
		default:

			// GetPodList（opts ... ListOption） If no value is specified, get all the namespace's pods
			AllPodList, err = f.GetPodList()
			if err != nil {
				return err
			}

			if f.CheckPodListRunning(AllPodList) {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}
