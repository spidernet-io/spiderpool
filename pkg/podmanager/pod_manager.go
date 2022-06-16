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
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

type PodManager interface {
	GetPodByName(ctx context.Context, namespace, podName string) (*corev1.Pod, error)
	GetOwnerType(ctx context.Context, pod *corev1.Pod) types.OwnerType
	IsIPAllocatable(ctx context.Context, pod *corev1.Pod) (types.PodStatus, bool)
	MergeAnnotations(ctx context.Context, pod *corev1.Pod, annotations map[string]string) error
	MatchLabelSelector(ctx context.Context, namespace, podName string, labelSelector *metav1.LabelSelector) (bool, error)
}

type podManager struct {
	client            client.Client
	runtimeMgr        ctrl.Manager
	maxConflictRetrys int
}

func NewPodManager(c client.Client, mgr ctrl.Manager, maxConflictRetrys int) (PodManager, error) {
	if c == nil {
		return nil, errors.New("k8s client must be specified")
	}

	if mgr == nil {
		return nil, errors.New("runtime manager must be specified")
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &corev1.Pod{}, metav1.ObjectNameField, func(raw client.Object) []string {
		pod := raw.(*corev1.Pod)
		return []string{pod.Name}
	}); err != nil {
		return nil, err
	}

	return &podManager{
		client:            c,
		runtimeMgr:        mgr,
		maxConflictRetrys: maxConflictRetrys,
	}, nil
}

func (r *podManager) GetPodByName(ctx context.Context, namespace, podName string) (*corev1.Pod, error) {
	var pod corev1.Pod
	if err := r.client.Get(ctx, apitypes.NamespacedName{Namespace: namespace, Name: podName}, &pod); err != nil {
		return nil, err
	}

	return &pod, nil
}

func (r *podManager) GetOwnerType(ctx context.Context, pod *corev1.Pod) types.OwnerType {
	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		return constant.OwnerNone
	}

	var ownerType types.OwnerType
	switch types.OwnerType(owner.Kind) {
	case constant.OwnerDeployment:
		ownerType = constant.OwnerDeployment
	case constant.OwnerStatefuleSet:
		ownerType = constant.OwnerStatefuleSet
	case constant.OwnerDaemonSet:
		ownerType = constant.OwnerDaemonSet
	default:
		ownerType = constant.OwnerCRD
	}

	return ownerType
}

func (r *podManager) IsIPAllocatable(ctx context.Context, pod *corev1.Pod) (types.PodStatus, bool) {
	if pod.DeletionTimestamp != nil && pod.DeletionGracePeriodSeconds != nil {
		now := time.Now()
		deletionTime := pod.DeletionTimestamp.Time
		deletionGracePeriod := time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second
		if now.After(deletionTime.Add(deletionGracePeriod)) {
			return constant.PodTerminating, false
		}
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

func (r *podManager) MergeAnnotations(ctx context.Context, pod *corev1.Pod, annotations map[string]string) error {
	merge := map[string]string{}
	for k, v := range pod.Annotations {
		merge[k] = v
	}

	for k, v := range annotations {
		merge[k] = v
	}

	pod.Annotations = merge

	rand.Seed(time.Now().UnixNano())
	for i := 0; i <= r.maxConflictRetrys; i++ {
		if err := r.client.Update(ctx, pod); err != nil {
			if apierrors.IsConflict(err) {
				if i == r.maxConflictRetrys {
					return fmt.Errorf("insufficient retries(<=%d) to merge Pod annotations", r.maxConflictRetrys)
				}

				time.Sleep(time.Duration(rand.Intn(1<<(i+1))) * time.Second)
				continue
			}
			return err
		}
		break
	}

	return nil
}

func (r *podManager) MatchLabelSelector(ctx context.Context, namespace, podName string, labelSelector *metav1.LabelSelector) (bool, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return false, err
	}

	var pods corev1.PodList
	err = r.client.List(
		ctx,
		&pods,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
		client.MatchingFields{metav1.ObjectNameField: podName},
	)
	if err != nil {
		return false, err
	}

	if len(pods.Items) == 0 {
		return false, nil
	}

	return true, nil
}
