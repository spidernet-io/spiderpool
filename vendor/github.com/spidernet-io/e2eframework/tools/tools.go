// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package tools

import (
	"github.com/asaskevich/govalidator"
	corev1 "k8s.io/api/core/v1"

	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"fmt"
	"time"
)

func CheckPodIpv4IPReady(pod *corev1.Pod) (string, bool) {
	if pod == nil {
		return "", false
	}
	for _, v := range pod.Status.PodIPs {
		if govalidator.IsIPv4(v.IP) {
			return v.IP, true
		}
	}
	return "", false
}

func CheckPodIpv6IPReady(pod *corev1.Pod) (string, bool) {
	if pod == nil {
		return "", false
	}
	for _, v := range pod.Status.PodIPs {
		if govalidator.IsIPv6(v.IP) {
			return v.IP, true
		}
	}
	return "", false
}

func RandomName() string {
	m := time.Now()
	return fmt.Sprintf("%v%v-%v", m.Minute(), m.Second(), m.Nanosecond())
}

// simulate Eventually for internal
func Eventually(f func() bool, timeout time.Duration, interval time.Duration) bool {
	timeoutAfter := time.After(timeout)
	for {
		select {
		case <-timeoutAfter:
			return false
		default:
		}
		if f() {
			return true
		}
		time.Sleep(interval)
	}
}
