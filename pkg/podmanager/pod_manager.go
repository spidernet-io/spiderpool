// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"errors"
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
)

type PodManager interface {
	GetPodByName(ctx context.Context, namespace, podName string) (*corev1.Pod, error)
	ListPods(ctx context.Context, opts ...client.ListOption) (*corev1.PodList, error)
	MergeAnnotations(ctx context.Context, namespace, podName string, annotations map[string]string) error
	MatchLabelSelector(ctx context.Context, namespace, podName string, labelSelector *metav1.LabelSelector) (bool, error)
	GetPodTopController(ctx context.Context, pod *corev1.Pod) (appKind string, app metav1.Object, err error)
}

type podManager struct {
	config *PodManagerConfig
	client client.Client
}

func NewPodManager(c *PodManagerConfig, client client.Client) (PodManager, error) {
	if c == nil {
		return nil, errors.New("pod manager config must be specified")
	}
	if client == nil {
		return nil, errors.New("k8s client must be specified")
	}

	return &podManager{
		config: c,
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

func (pm *podManager) MergeAnnotations(ctx context.Context, namespace, podName string, annotations map[string]string) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= pm.config.MaxConflictRetrys; i++ {
		pod, err := pm.GetPodByName(ctx, namespace, podName)
		if err != nil {
			return err
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
			if i == pm.config.MaxConflictRetrys {
				return fmt.Errorf("insufficient retries(<=%d) to merge Pod annotations", pm.config.MaxConflictRetrys)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * pm.config.ConflictRetryUnitTime)
			continue
		}
		break
	}

	return nil
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

// GetPodTopController will find the pod top owner controller with the given pod.
// For example, once we create a deployment then it will create replicaset and the replicaset will create pods.
// So, the pods' top owner is deployment. That's what the method implements.
func (pm *podManager) GetPodTopController(ctx context.Context, pod *corev1.Pod) (appKind string, app metav1.Object, err error) {
	logger := logutils.FromContext(ctx)
	var ownerErr = fmt.Errorf("failed to get pod '%s/%s' owner", pod.Namespace, pod.Name)

	podOwner := metav1.GetControllerOf(pod)
	if podOwner == nil {
		return constant.OwnerNone, nil, nil
	}

	namespacedName := apitypes.NamespacedName{
		Namespace: pod.Namespace,
		Name:      podOwner.Name,
	}

	switch podOwner.Kind {
	case constant.OwnerReplicaSet:
		var replicaset appsv1.ReplicaSet
		err = pm.client.Get(ctx, namespacedName, &replicaset)
		if nil != err {
			return "", nil, fmt.Errorf("%w: %v", ownerErr, err)
		}

		replicasetOwner := metav1.GetControllerOf(&replicaset)
		if replicasetOwner != nil {
			if replicasetOwner.Kind == constant.OwnerDeployment {
				var deployment appsv1.Deployment
				err = pm.client.Get(ctx, apitypes.NamespacedName{Namespace: replicaset.Namespace, Name: replicasetOwner.Name}, &deployment)
				if nil != err {
					return "", nil, fmt.Errorf("%w: %v", ownerErr, err)
				}
				return constant.OwnerDeployment, &deployment, nil
			}

			logger.Sugar().Warnf("the controller type '%s' of pod '%s/%s' is unknown", replicasetOwner.Kind, pod.Namespace, pod.Name)
			return constant.OwnerUnknown, nil, nil
		}
		return constant.OwnerReplicaSet, &replicaset, nil

	case constant.OwnerJob:
		var job batchv1.Job
		err = pm.client.Get(ctx, namespacedName, &job)
		if nil != err {
			return "", nil, fmt.Errorf("%w: %v", ownerErr, err)
		}
		jobOwner := metav1.GetControllerOf(&job)
		if jobOwner != nil {
			if jobOwner.Kind == constant.OwnerCronJob {
				var cronJob batchv1.CronJob
				err = pm.client.Get(ctx, apitypes.NamespacedName{Namespace: job.Namespace, Name: jobOwner.Name}, &cronJob)
				if nil != err {
					return "", nil, fmt.Errorf("%w: %v", ownerErr, err)
				}
				return constant.OwnerCronJob, &cronJob, nil
			}

			logger.Sugar().Warnf("the controller type '%s' of pod '%s/%s' is unknown", jobOwner.Kind, pod.Namespace, pod.Name)
			return constant.OwnerUnknown, nil, nil
		}
		return constant.OwnerJob, &job, nil

	case constant.OwnerDaemonSet:
		var daemonSet appsv1.DaemonSet
		err = pm.client.Get(ctx, namespacedName, &daemonSet)
		if nil != err {
			return "", nil, fmt.Errorf("%w: %v", ownerErr, err)
		}
		return constant.OwnerDaemonSet, &daemonSet, nil

	case constant.OwnerStatefulSet:
		var statefulSet appsv1.StatefulSet
		err = pm.client.Get(ctx, namespacedName, &statefulSet)
		if nil != err {
			return "", nil, fmt.Errorf("%w: %v", ownerErr, err)
		}
		return constant.OwnerStatefulSet, &statefulSet, nil
	}

	logger.Sugar().Warnf("the controller type '%s' of pod '%s/%s' is unknown", podOwner.Kind, pod.Namespace, pod.Name)
	return constant.OwnerUnknown, nil, nil
}
