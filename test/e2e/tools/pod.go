// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"github.com/asaskevich/govalidator"
	corev1 "k8s.io/api/core/v1"
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
