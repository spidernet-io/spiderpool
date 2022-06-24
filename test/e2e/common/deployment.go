// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

func GenerateExampleDeploymentYaml(dpmName, namespace string, replica int32) *appsv1.Deployment {
	Expect(dpmName).NotTo(BeEmpty())
	Expect(namespace).NotTo(BeEmpty())

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      dpmName,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(replica),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": dpmName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": dpmName,
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

func ScaleDeployUntilExpectedReplicas(frame *e2e.Framework, deploy *appsv1.Deployment, expectedReplicas int, ctx context.Context) (addedPod, removedPod []corev1.Pod, err error) {

	if frame == nil || deploy == nil || expectedReplicas <= 0 || int32(expectedReplicas) == *deploy.Spec.Replicas {
		return nil, nil, e2e.ErrWrongInput
	}

	var newPodList *corev1.PodList

	podList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
	Expect(err).NotTo(HaveOccurred())
	GinkgoWriter.Printf("deloyment %v/%v scale replicas from %v to %v \n", deploy.Namespace, deploy.Name, len(podList.Items), expectedReplicas)

	deploy, err = frame.ScaleDeployment(deploy, int32(expectedReplicas))
	Expect(err).NotTo(HaveOccurred())
	Expect(*deploy.Spec.Replicas).To(Equal(int32(expectedReplicas)))
	GinkgoWriter.Printf("Successful scale order to start waiting for expected replicas: %v \n", expectedReplicas)

	for {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("time out to wait expected replicas: %v ", expectedReplicas)
		default:
			newPodList, err = frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
			Expect(err).NotTo(HaveOccurred())
			if len(newPodList.Items) != expectedReplicas {
				break
			}
			// return the diff pod
			if expectedReplicas > len(podList.Items) {
				addedPod := GetAdditionalPods(podList, newPodList)
				return addedPod, nil, nil
			}
			if expectedReplicas < len(podList.Items) {
				removedPod := GetAdditionalPods(newPodList, podList)
				return nil, removedPod, nil
			}
		}
		time.Sleep(time.Second)
	}
}
