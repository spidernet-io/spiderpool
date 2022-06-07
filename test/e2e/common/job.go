// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

type JobBehave string

const (
	JobTypeRunningForever JobBehave = "runningForeverJob"
	JobTypeFail           JobBehave = "failedJob"
	JobTypeFinish         JobBehave = "succeedJob"
)

func GenerateExampleJobYaml(behavior JobBehave, jdName, namespace string, parallelism *int32) *batchv1.Job {

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
							Command:         []string{},
						},
					},
				},
			},
		},
	}

	switch behavior {
	case JobTypeRunningForever:
		jobYaml.Spec.Template.Spec.Containers[0].Command = []string{"sleep", "infinity"}
	case JobTypeFail:
		jobYaml.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "exit 1"}
	case JobTypeFinish:
		jobYaml.Spec.Template.Spec.Containers[0].Command = []string{"/bin/sh", "-c", "exit 0"}
	default:
		GinkgoWriter.Printf("input error\n")
		return nil
	}

	return jobYaml
}
