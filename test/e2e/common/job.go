// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func GenerateExampleJobYaml(behavior, jdName, namespace string, parallelism *int32) *batchv1.Job {
	Expect(jdName).NotTo(BeEmpty())
	Expect(namespace).NotTo(BeEmpty())

	jobYaml := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      jdName,
		},
		TypeMeta: metav1.TypeMeta{
			Kind: "Job",
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:   pointer.Int32Ptr(0),
			Parallelism:    parallelism,
			ManualSelector: pointer.Bool(true),

			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": jdName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": jdName,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: "Never",
					Containers: []corev1.Container{
						{
							Name:            "samplepod",
							Image:           "alpine",
							ImagePullPolicy: "IfNotPresent",
							Command:         []string{"/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"},
						},
					},
				},
			},
		},
	}

	switch behavior {
	case "notTerminate":
		jobYaml.Spec.Template.Spec.Containers[0].Command = []string{"sleep", "1000000"}
	case "failed":
		jobYaml.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "exit 1"}
	case "succeeded":
		jobYaml.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "exit 0"}
	}

	return jobYaml
}
