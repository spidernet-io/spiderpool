// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func GetOwnerControllerType(pod *corev1.Pod) (string, string) {
	owner := metav1.GetControllerOf(pod)
	if owner == nil {
		return constant.OwnerNone, ""
	}

	var ownerType string
	switch owner.Kind {
	case constant.OwnerDeployment:
		ownerType = constant.OwnerDeployment
	case constant.OwnerStatefulSet:
		ownerType = constant.OwnerStatefulSet
	case constant.OwnerDaemonSet:
		ownerType = constant.OwnerDaemonSet
	case constant.OwnerReplicaSet:
		ownerType = constant.OwnerReplicaSet
	case constant.OwnerJob:
		ownerType = constant.OwnerJob
	default:
		ownerType = constant.OwnerUnknown
	}

	return ownerType, owner.Name
}

func CheckPodStatus(pod *corev1.Pod) (podStatue types.PodStatus, isAllocatable bool) {
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
