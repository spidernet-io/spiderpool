// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"github.com/asaskevich/govalidator"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CheckPodIpv4IPReady(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, v := range pod.Status.PodIPs {
		if govalidator.IsIPv4(v.IP) {
			return true
		}
	}
	return false
}

func CheckPodIpv6IPReady(pod *corev1.Pod) bool {
	if pod == nil {
		return false
	}
	for _, v := range pod.Status.PodIPs {
		if govalidator.IsIPv6(v.IP) {
			return true
		}
	}
	return false
}

func GenerateExamplePodYaml(podName, namespace string) *corev1.Pod {
	Expect(podName).NotTo(BeEmpty())
	Expect(namespace).NotTo(BeEmpty())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "samplepod",
					Image:           "alpine",
					ImagePullPolicy: "IfNotPresent",
					Command:         []string{"/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"},
				},
			},
		},
	}
}
