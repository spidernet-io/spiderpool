// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type PodManager interface {
	GetPodByName(ctx context.Context, namespace, podName string) (*corev1.Pod, error)
	ListPods(ctx context.Context, opts ...client.ListOption) (*corev1.PodList, error)
	MatchLabelSelector(ctx context.Context, namespace, podName string, labelSelector *metav1.LabelSelector) (bool, error)
	MergeAnnotations(ctx context.Context, namespace, podName string, annotations map[string]string) error
	GetPodTopController(ctx context.Context, pod *corev1.Pod) (types.PodTopController, error)
}

type podManager struct {
	config PodManagerConfig
	client client.Client
}

func NewPodManager(config PodManagerConfig, client client.Client) (PodManager, error) {
	if client == nil {
		return nil, fmt.Errorf("k8s client %w", constant.ErrMissingRequiredParam)
	}

	return &podManager{
		config: setDefaultsForPodManagerConfig(config),
		client: client,
	}, nil
}

func (pm *podManager) GetPodByName(ctx context.Context, namespace, podName string) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := pm.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}

func (pm *podManager) ListPods(ctx context.Context, opts ...client.ListOption) (*corev1.PodList, error) {
	var podList corev1.PodList
	if err := pm.client.List(ctx, &podList, opts...); err != nil {
		return nil, err
	}

	return &podList, nil
}

func (pm *podManager) MatchLabelSelector(ctx context.Context, namespace, podName string, labelSelector *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, err
	}

	podList, err := pm.ListPods(
		ctx,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
		client.MatchingFields{metav1.ObjectNameField: podName},
	)
	if err != nil {
		return false, err
	}

	if len(podList.Items) == 0 {
		return false, nil
	}

	return true, nil
}

func (pm *podManager) MergeAnnotations(ctx context.Context, namespace, podName string, annotations map[string]string) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= pm.config.MaxConflictRetries; i++ {
		pod, err := pm.GetPodByName(ctx, namespace, podName)
		if err != nil {
			return err
		}

		if len(annotations) == 0 {
			return nil
		}

		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}

		for k, v := range annotations {
			pod.Annotations[k] = v
		}
		if err := pm.client.Update(ctx, pod); err != nil {
			if !apierrors.IsConflict(err) {
				return err
			}
			if i == pm.config.MaxConflictRetries {
				return fmt.Errorf("%w (%d times), failed to merge Pod annotations", constant.ErrRetriesExhausted, pm.config.MaxConflictRetries)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * pm.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
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
			Kind:      constant.KindPod,
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Uid:       pod.UID,
			App:       pod,
		}, nil
	}

	// third party controller
	if podOwner.APIVersion != appsv1.SchemeGroupVersion.String() && podOwner.APIVersion != batchv1.SchemeGroupVersion.String() {
		return types.PodTopController{
			Kind:      constant.KindUnknown,
			Namespace: pod.Namespace,
			Name:      podOwner.Name,
			Uid:       podOwner.UID,
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
		if replicasetOwner != nil {
			if replicasetOwner.Kind == constant.KindDeployment {
				var deployment appsv1.Deployment
				err = pm.client.Get(ctx, apitypes.NamespacedName{Namespace: replicaset.Namespace, Name: replicasetOwner.Name}, &deployment)
				if nil != err {
					return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
				}
				return types.PodTopController{
					Kind:      constant.KindDeployment,
					Namespace: deployment.Namespace,
					Name:      deployment.Name,
					Uid:       deployment.UID,
					App:       &deployment,
				}, nil
			}

			logger.Sugar().Warnf("the controller type '%s' of pod '%s/%s' is unknown", replicasetOwner.Kind, pod.Namespace, pod.Name)
			return types.PodTopController{
				Kind:      constant.KindUnknown,
				Namespace: pod.Namespace,
				Name:      replicasetOwner.Name,
				Uid:       replicasetOwner.UID,
			}, nil
		}
		return types.PodTopController{
			Kind:      constant.KindReplicaSet,
			Namespace: replicaset.Namespace,
			Name:      replicaset.Name,
			Uid:       replicaset.UID,
			App:       &replicaset,
		}, nil

	case constant.KindJob:
		var job batchv1.Job
		err := pm.client.Get(ctx, namespacedName, &job)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		jobOwner := metav1.GetControllerOf(&job)
		if jobOwner != nil {
			if jobOwner.Kind == constant.KindCronJob {
				var cronJob batchv1.CronJob
				err = pm.client.Get(ctx, apitypes.NamespacedName{Namespace: job.Namespace, Name: jobOwner.Name}, &cronJob)
				if nil != err {
					return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
				}
				return types.PodTopController{
					Kind:      constant.KindCronJob,
					Namespace: cronJob.Namespace,
					Name:      cronJob.Name,
					Uid:       cronJob.UID,
					App:       &cronJob,
				}, nil
			}

			logger.Sugar().Warnf("the controller type '%s' of pod '%s/%s' is unknown", jobOwner.Kind, pod.Namespace, pod.Name)
			return types.PodTopController{
				Kind:      constant.KindUnknown,
				Namespace: job.Namespace,
				Name:      jobOwner.Name,
				Uid:       jobOwner.UID,
			}, nil
		}
		return types.PodTopController{
			Kind:      constant.KindJob,
			Namespace: job.Namespace,
			Name:      job.Name,
			Uid:       job.UID,
			App:       &job,
		}, nil

	case constant.KindDaemonSet:
		var daemonSet appsv1.DaemonSet
		err := pm.client.Get(ctx, namespacedName, &daemonSet)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		return types.PodTopController{
			Kind:      constant.KindDaemonSet,
			Namespace: daemonSet.Namespace,
			Name:      daemonSet.Name,
			Uid:       daemonSet.UID,
			App:       &daemonSet,
		}, nil

	case constant.KindStatefulSet:
		var statefulSet appsv1.StatefulSet
		err := pm.client.Get(ctx, namespacedName, &statefulSet)
		if nil != err {
			return types.PodTopController{}, fmt.Errorf("%w: %v", ownerErr, err)
		}
		return types.PodTopController{
			Kind:      constant.KindStatefulSet,
			Namespace: statefulSet.Namespace,
			Name:      statefulSet.Name,
			Uid:       statefulSet.UID,
			App:       &statefulSet,
		}, nil
	}

	logger.Sugar().Warnf("the controller type '%s' of pod '%s/%s' is unknown", podOwner.Kind, pod.Namespace, pod.Name)
	return types.PodTopController{
		Kind:      constant.KindUnknown,
		Namespace: pod.Namespace,
		Name:      podOwner.Name,
		Uid:       podOwner.UID,
	}, nil
}
