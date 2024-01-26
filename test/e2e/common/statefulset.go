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
	"k8s.io/utils/ptr"
)

func GenerateExampleStatefulSetYaml(stsName, namespace string, replica int32) *appsv1.StatefulSet {
	Expect(stsName).NotTo(BeEmpty())
	Expect(namespace).NotTo(BeEmpty())

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      stsName,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: ptr.To(replica),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": stsName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": stsName,
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

func ScaleStatefulsetUntilExpectedReplicas(ctx context.Context, frame *e2e.Framework, sts *appsv1.StatefulSet, expectedReplicas int, scalePodRun bool) (addedPod, removedPod []corev1.Pod, err error) {
	if frame == nil || sts == nil || expectedReplicas <= 0 || int32(expectedReplicas) == *sts.Spec.Replicas {
		return nil, nil, e2e.ErrWrongInput
	}
	var newPodList *corev1.PodList

	podList, err := frame.GetPodListByLabel(sts.Spec.Selector.MatchLabels)
	Expect(err).NotTo(HaveOccurred())
	GinkgoWriter.Printf("Statefulset %v/%v scale replicas from %v to %v \n", sts.Namespace, sts.Name, len(podList.Items), expectedReplicas)

	sts, err = frame.ScaleStatefulSet(sts, int32(expectedReplicas))
	Expect(err).NotTo(HaveOccurred())
	Expect(*sts.Spec.Replicas).To(Equal(int32(expectedReplicas)))

	for {
		select {
		case <-ctx.Done():
			return nil, nil, fmt.Errorf("time out to wait expected replicas: %v ", expectedReplicas)
		default:
			newPodList, err = frame.GetPodListByLabel(sts.Spec.Selector.MatchLabels)
			Expect(err).NotTo(HaveOccurred())
			if len(newPodList.Items) != expectedReplicas {
				break
			}
			if scalePodRun && !frame.CheckPodListRunning(newPodList) {
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
		time.Sleep(ForcedWaitingTime)
	}
}
