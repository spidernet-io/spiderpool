// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	corev1 "k8s.io/api/core/v1"
)

func IsPodAlive(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}

	if pod.DeletionTimestamp != nil {
		return false
	}

	if pod.Status.Phase == corev1.PodSucceeded && pod.Spec.RestartPolicy != corev1.RestartPolicyAlways {
		return false
	}

	if pod.Status.Phase == corev1.PodFailed && pod.Spec.RestartPolicy == corev1.RestartPolicyNever {
		return false
	}

	if pod.Status.Phase == corev1.PodFailed && pod.Status.Reason == "Evicted" {
		return false
	}

	return true
}
