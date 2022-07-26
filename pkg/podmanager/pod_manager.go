// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type PodManager interface {
	GetPodByName(ctx context.Context, namespace, podName string) (*corev1.Pod, error)
	ListPods(ctx context.Context, opts ...client.ListOption) (*corev1.PodList, error)
	CheckPodStatus(pod *corev1.Pod) (types.PodStatus, bool)
	MergeAnnotations(ctx context.Context, namespace, podName string, annotations map[string]string) error
	MatchLabelSelector(ctx context.Context, namespace, podName string, labelSelector *metav1.LabelSelector) (bool, error)
}

type podManager struct {
	client                client.Client
	maxConflictRetrys     int
	conflictRetryUnitTime time.Duration
}

func NewPodManager(c client.Client, maxConflictRetrys int, conflictRetryUnitTime time.Duration) (PodManager, error) {
	if c == nil {
		return nil, errors.New("k8s client must be specified")
	}

	return &podManager{
		client:                c,
		maxConflictRetrys:     maxConflictRetrys,
		conflictRetryUnitTime: conflictRetryUnitTime,
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

func (pm *podManager) CheckPodStatus(pod *corev1.Pod) (podStatue types.PodStatus, isAllocatable bool) {
	// TODO (Icarus9913): no pending phase?
	if pod.DeletionTimestamp != nil && pod.DeletionGracePeriodSeconds != nil {
		now := time.Now()
		deletionTime := pod.DeletionTimestamp.Time
		deletionGracePeriod := time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second
		if now.After(deletionTime.Add(deletionGracePeriod)) {
			return constant.PodGraceTimeOut, false
		}
		return constant.PodTerminating, false
	}

	if pod.Status.Phase == corev1.PodSucceeded && pod.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		return constant.PodSucceeded, false
	}

	if pod.Status.Phase == corev1.PodFailed && pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
		return constant.PodFailed, false
	}

	if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted" {
		return constant.PodEvicted, false
	}

	return constant.PodRunning, true
}

func (pm *podManager) MergeAnnotations(ctx context.Context, namespace, podName string, annotations map[string]string) error {
	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= pm.maxConflictRetrys; i++ {
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
			if i == pm.maxConflictRetrys {
				return fmt.Errorf("insufficient retries(<=%d) to merge Pod annotations", pm.maxConflictRetrys)
			}
			time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * pm.conflictRetryUnitTime)
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
