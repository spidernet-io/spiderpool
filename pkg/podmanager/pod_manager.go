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
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/strings/slices"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type PodManager interface {
	GetPodByName(ctx context.Context, namespace, podName string, cached bool) (*corev1.Pod, error)
	ListPods(ctx context.Context, cached bool, opts ...client.ListOption) (*corev1.PodList, error)
	GetPodTopController(ctx context.Context, pod *corev1.Pod) (types.PodTopController, error)
}

type podManager struct {
	client    client.Client
	apiReader client.Reader
}

func NewPodManager(client client.Client, apiReader client.Reader) (PodManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}
	if apiReader == nil {
		return nil, fmt.Errorf("api reader %w", constant.ErrMissingRequiredParam)
	}

	// test ci
	return &podManager{
		client:    client,
		apiReader: apiReader,
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
// Notice: if the application is a third party controller, the types.PodTopController property App would be nil!
func (pm *podManager) GetPodTopController(ctx context.Context, pod *corev1.Pod) (types.PodTopController, error) {
	logger := logutils.FromContext(ctx)

	var ownerErr = fmt.Errorf("failed to get pod '%s/%s' owner", pod.Namespace, pod.Name)

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
			UID: pod.UID,
			APP: pod,
		}, nil
	}

	// third party controller
	if !slices.Contains(constant.K8sAPIVersions, podOwner.APIVersion) {
		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				APIVersion: podOwner.APIVersion,
				Kind:       podOwner.Kind,
				Namespace:  pod.Namespace,
				Name:       podOwner.Name,
			},
			UID: podOwner.UID,
		}, nil
	}

	namespacedName := apitypes.NamespacedName{
		Namespace: pod.Namespace,
		Name:      podOwner.Name,
	}

	switch podOwner.Kind {
	case constant.KindReplicaSet:
		var replicaset appsv1.ReplicaSet
		err := pm.client.Get(ctx, namespacedName, &replicaset)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}

		replicasetOwner := metav1.GetControllerOf(&replicaset)
		if replicasetOwner != nil && replicasetOwner.APIVersion == appsv1.SchemeGroupVersion.String() && replicasetOwner.Kind == constant.KindDeployment {
			var deployment appsv1.Deployment
			err = pm.client.Get(ctx, apitypes.NamespacedName{Namespace: replicaset.Namespace, Name: replicasetOwner.Name}, &deployment)
			if nil != err {
				return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
			}
			return types.PodTopController{
				AppNamespacedName: types.AppNamespacedName{
					// deployment.APIVersion is empty string
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       constant.KindDeployment,
					Namespace:  deployment.Namespace,
					Name:       deployment.Name,
				},
				UID: deployment.UID,
				APP: &deployment,
			}, nil
		}

		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindReplicaSet,
				Namespace:  replicaset.Namespace,
				Name:       replicaset.Name,
			},
			UID: replicaset.UID,
			APP: &replicaset,
		}, nil

	case constant.KindJob:
		var job batchv1.Job
		err := pm.client.Get(ctx, namespacedName, &job)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		jobOwner := metav1.GetControllerOf(&job)
		if jobOwner != nil && jobOwner.APIVersion == batchv1.SchemeGroupVersion.String() && jobOwner.Kind == constant.KindCronJob {
			var cronJob batchv1.CronJob
			err = pm.client.Get(ctx, apitypes.NamespacedName{Namespace: job.Namespace, Name: jobOwner.Name}, &cronJob)
			if nil != err {
				return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
			}
			return types.PodTopController{
				AppNamespacedName: types.AppNamespacedName{
					APIVersion: batchv1.SchemeGroupVersion.String(),
					Kind:       constant.KindCronJob,
					Namespace:  cronJob.Namespace,
					Name:       cronJob.Name,
				},
				UID: cronJob.UID,
				APP: &cronJob,
			}, nil
		}

		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				APIVersion: batchv1.SchemeGroupVersion.String(),
				Kind:       constant.KindJob,
				Namespace:  job.Namespace,
				Name:       job.Name,
			},
			UID: job.UID,
			APP: &job,
		}, nil

	case constant.KindDaemonSet:
		var daemonSet appsv1.DaemonSet
		err := pm.client.Get(ctx, namespacedName, &daemonSet)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				// daemonSet.APIVersion is empty string
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindDaemonSet,
				Namespace:  daemonSet.Namespace,
				Name:       daemonSet.Name,
			},
			UID: daemonSet.UID,
			APP: &daemonSet,
		}, nil

	case constant.KindStatefulSet:
		var statefulSet appsv1.StatefulSet
		err := pm.client.Get(ctx, namespacedName, &statefulSet)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		return types.PodTopController{
			AppNamespacedName: types.AppNamespacedName{
				// statefulSet.APIVersion is empty string
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       constant.KindStatefulSet,
				Namespace:  statefulSet.Namespace,
				Name:       statefulSet.Name,
			},
			UID: statefulSet.UID,
			APP: &statefulSet,
		}, nil
	}

	logger.Sugar().Warnf("the controller type '%s' of pod '%s/%s' is unknown", podOwner.Kind, pod.Namespace, pod.Name)
	return types.PodTopController{
		AppNamespacedName: types.AppNamespacedName{
			APIVersion: podOwner.APIVersion,
			Kind:       podOwner.Kind,
			Namespace:  pod.Namespace,
			Name:       podOwner.Name,
		},
		UID: podOwner.UID,
	}, nil
}
