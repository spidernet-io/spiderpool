// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/pointer"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type PodManager interface {
	GetPodByName(ctx context.Context, namespace, podName string, cached bool) (*corev1.Pod, error)
	ListPods(ctx context.Context, cached bool, opts ...client.ListOption) (*corev1.PodList, error)
	GetPodTopController(ctx context.Context, pod *corev1.Pod) (types.PodTopController, error)
}

type podManager struct {
	client        client.Client
	apiReader     client.Reader
	dynamicClient dynamic.Interface
}

func NewPodManager(client client.Client, apiReader client.Reader, dynamicClient dynamic.Interface) (PodManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}
	if dynamicClient == nil {
		return nil, fmt.Errorf("dynamic client %w", constant.ErrMissingRequiredParam)
	}

	return &podManager{
		client:        client,
		apiReader:     apiReader,
		dynamicClient: dynamicClient,
	}, nil
}

func (pm *podManager) GetPodByName(ctx context.Context, namespace, podName string, cached bool) (*corev1.Pod, error) {
	reader := pm.apiReader
	if cached == constant.UseCache {
		reader = pm.client
	}

	var pod corev1.Pod
	if err := reader.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}

func (pm *podManager) ListPods(ctx context.Context, cached bool, opts ...client.ListOption) (*corev1.PodList, error) {
	reader := pm.apiReader
	if cached == constant.UseCache {
		reader = pm.client
	}

	var podList corev1.PodList
	if err := reader.List(ctx, &podList, opts...); err != nil {
		return nil, err
	}

	return &podList, nil
}

// GetPodTopController will find the pod top owner controller with the given pod.
// For example, once we create a deployment then it will create replicaset and the replicaset will create pods.
// So, the pods' top owner is deployment. That's what the method implements.
// Notice: if the application is a third party controller, the types.PodTopController property Replicas would be nil!
func (pm *podManager) GetPodTopController(ctx context.Context, pod *corev1.Pod) (types.PodTopController, error) {
	podOwner := metav1.GetControllerOf(pod)
	if podOwner == nil {
		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				// pod.APIVersion is empty string
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       constant.KindPod,
				Namespace:  pod.Namespace,
				Name:       pod.Name,
			},
			UID:      pod.UID,
			Replicas: pointer.Int(1),
		}, nil
	}

	unstructuredObj, ownerReference, err := pm.controller(ctx, podOwner, pod.Namespace)
	if nil != err {
		return types.PodTopController{}, fmt.Errorf("failed to get pod top controller, error: %w", err)
	}

	replicas, err := getK8sKindControllerReplicas(unstructuredObj, ownerReference.APIVersion, ownerReference.Kind)
	if nil != err {
		return types.PodTopController{}, fmt.Errorf("failed to get controller '%s/%s' replicas, error: %w", ownerReference.APIVersion, ownerReference.Kind, err)
	}

	result := types.PodTopController{
		AppNamespacedName: types.AppNamespacedName{
			APIVersion: ownerReference.APIVersion,
			Kind:       ownerReference.Kind,
			Namespace:  pod.Namespace,
			Name:       ownerReference.Name,
		},
		UID:      ownerReference.UID,
		Replicas: replicas,
	}

	return result, nil
}

func (pm *podManager) controller(ctx context.Context, controllerOwnerRef *metav1.OwnerReference, namespace string) (*unstructured.Unstructured, *metav1.OwnerReference, error) {
	gvr, err := applicationinformers.GenerateGVR(controllerOwnerRef.APIVersion, controllerOwnerRef.Kind)
	if nil != err {
		return nil, nil, err
	}
	unstructuredObj, err := pm.dynamicClient.Resource(gvr).Namespace(namespace).Get(ctx, controllerOwnerRef.Name, metav1.GetOptions{})
	if nil != err {
		return nil, nil, err
	}
	owner := metav1.GetControllerOf(unstructuredObj)
	if owner == nil {
		return unstructuredObj, controllerOwnerRef, nil
	}

	return pm.controller(ctx, owner, namespace)
}

func getK8sKindControllerReplicas(unstructuredObj *unstructured.Unstructured, apiVersion, kind string) (*int, error) {
	if slices.Contains(constant.K8sAPIVersions, apiVersion) {
		switch kind {
		case constant.KindDeployment:
			var deployment appsv1.Deployment
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &deployment)
			if nil != err {
				return nil, err
			}
			return pointer.Int(applicationinformers.GetAppReplicas(deployment.Spec.Replicas)), nil

		case constant.KindReplicaSet:
			var replicaSet appsv1.ReplicaSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &replicaSet)
			if nil != err {
				return nil, err
			}
			return pointer.Int(applicationinformers.GetAppReplicas(replicaSet.Spec.Replicas)), nil

		case constant.KindStatefulSet:
			var statefulSet appsv1.ReplicaSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &statefulSet)
			if nil != err {
				return nil, err
			}
			return pointer.Int(applicationinformers.GetAppReplicas(statefulSet.Spec.Replicas)), nil

		case constant.KindDaemonSet:
			var daemonSet appsv1.DaemonSet
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &daemonSet)
			if nil != err {
				return nil, err
			}
			return pointer.Int(int(daemonSet.Status.DesiredNumberScheduled)), nil

		case constant.KindJob:
			var job batchv1.Job
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &job)
			if nil != err {
				return nil, err
			}
			return pointer.Int(applicationinformers.CalculateJobPodNum(job.Spec.Parallelism, job.Spec.Completions)), nil

		case constant.KindCronJob:
			var cronJob batchv1.CronJob
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredObj.UnstructuredContent(), &cronJob)
			if nil != err {
				return nil, err
			}
			return pointer.Int(applicationinformers.CalculateJobPodNum(cronJob.Spec.JobTemplate.Spec.Parallelism, cronJob.Spec.JobTemplate.Spec.Completions)), nil

		}
	}

	return nil, nil
}
