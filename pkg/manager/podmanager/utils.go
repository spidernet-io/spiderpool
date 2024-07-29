// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubevirtv1 "kubevirt.io/api/core/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
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

// IsStaticIPPod checks the given pod's controller ownerReference whether is StatefulSet or KubevirtVMI
func IsStaticIPPod(enableStatefulSet, enableKubevirtStaticIP bool, pod *corev1.Pod) bool {
	ownerReference := metav1.GetControllerOf(pod)
	if ownerReference == nil {
		return false
	}

	if enableStatefulSet && ownerReference.APIVersion == appsv1.SchemeGroupVersion.String() && ownerReference.Kind == constant.KindStatefulSet {
		return true
	}

	if enableKubevirtStaticIP && ownerReference.APIVersion == kubevirtv1.SchemeGroupVersion.String() && ownerReference.Kind == constant.KindKubevirtVMI {
		return true
	}

	return false
}
