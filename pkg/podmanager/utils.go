// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

func CheckPodStatus(pod *corev1.Pod) (status types.PodStatus, allocatable bool) {
	if pod == nil {
		return constant.PodUnknown, false
	}

	// TODO (Icarus9913): no pending phase?
	if pod.DeletionTimestamp != nil && pod.DeletionGracePeriodSeconds != nil {
		now := time.Now()
		deletionTime := pod.DeletionTimestamp.Time
		deletionGracePeriod := time.Duration(*pod.DeletionGracePeriodSeconds) * time.Second
		if now.After(deletionTime.Add(deletionGracePeriod)) {
			return constant.PodGraceTimeout, false
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
