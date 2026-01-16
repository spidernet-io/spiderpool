// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Package common provides test utilities for E2E tests.
package common

import (
	//nolint:all // Standard for Ginkgo/Gomega BDD testing framework
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateExampleDaemonSetYaml(dsName, namespace string) *appsv1.DaemonSet {
	Expect(dsName).NotTo(BeEmpty())
	Expect(namespace).NotTo(BeEmpty())

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      dsName,
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": dsName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": dsName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "samplepod",
							Image:           "alpine",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/bin/ash", "-c", "sleep infinity"},
						},
					},
				},
			},
		},
	}
}
