// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package framework

import (
	"context"
	"errors"
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

func (f *Framework) GetDeployment(name, namespace string) (*appsv1.Deployment, error) {

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

func (f *Framework) CreateDeploymentUntilReady(deployObj *appsv1.Deployment, timeOut time.Duration, opts ...client.CreateOption) (*appsv1.Deployment, error) {
	if deployObj == nil {
		return nil, ErrWrongInput
	}

	// create deployment
	err := f.CreateDeployment(deployObj, opts...)
	if err != nil {
		return nil, err
	}

	// wait deployment ready
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()
	deploy, e := f.WaitDeploymentReady(deployObj.Name, deployObj.Namespace, ctx)
	if e != nil {
		return nil, e
	}
	return deploy, nil
}

func (f *Framework) DeleteDeploymentUntilFinish(deployName, namespace string, timeOut time.Duration, opts ...client.DeleteOption) error {
	if deployName == "" || namespace == "" {
		return ErrWrongInput
	}
	// get deployment
	deployment, err1 := f.GetDeployment(deployName, namespace)
	if err1 != nil {
		return err1
	}
	// delete deployment
	err := f.DeleteDeployment(deployment.Name, deployment.Namespace, opts...)
	if err != nil {
		return err
	}
	// check delete deployment successfully
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()
	b, e := func() (bool, error) {
		for {
			select {
			case <-ctx.Done():
				return false, ErrTimeOut
			default:
				deployment, _ := f.GetDeployment(deployment.Name, deployment.Namespace)
				if deployment == nil {
					return true, nil
				}
				time.Sleep(time.Second)
			}
		}
	}()
	if b {
		// check PodList not exists by label
		err := f.WaitPodListDeleted(deployment.Namespace, deployment.Spec.Selector.MatchLabels, ctx)
		if err != nil {
			return err
		}
		return nil
	}
	return e
}

func (f *Framework) WaitDeploymentReadyAndCheckIP(depName string, nsName string, timeout time.Duration) (*corev1.PodList, error) {
	// waiting for Deployment replicas to complete
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	dep, e := f.WaitDeploymentReady(depName, nsName, ctx)
	if e != nil {
		return nil, e
	}

	// check pods created by Deployment，its assign ipv4 and ipv6 addresses success
	podlist, err := f.GetDeploymentPodList(dep)
	if err != nil {
		return nil, err
	}

	// check IP address allocation succeeded
	errip := f.CheckPodListIpReady(podlist)
	if errip != nil {
		return nil, errip
	}
	return podlist, errip
}

func (f *Framework) RestartDeploymentPodUntilReady(deployName, namespace string, timeOut time.Duration, opts ...client.DeleteOption) error {
	if deployName == "" || namespace == "" {
		return ErrWrongInput
	}

	deployment, err := f.GetDeployment(deployName, namespace)
	if deployment == nil {
		return errors.New("failed to get deployment")
	}
	if err != nil {
		return err
	}
	podList, err := f.GetDeploymentPodList(deployment)

	if len(podList.Items) == 0 {
		return errors.New("failed to get podList")
	}
	if err != nil {
		return err
	}
	_, err = f.DeletePodListUntilReady(podList, timeOut, opts...)
	if err != nil {
		return err
	}
	return nil
}
