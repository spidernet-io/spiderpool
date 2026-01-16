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
	"sigs.k8s.io/controller-runtime/pkg/client"
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

func PatchStatefulSet(frame *e2e.Framework, desiredStatefulSet, originalStatefulSet *appsv1.StatefulSet, opts ...client.PatchOption) error {
	if desiredStatefulSet == nil || frame == nil || originalStatefulSet == nil {
		return e2e.ErrWrongInput
	}

	mergePatch := client.MergeFrom(originalStatefulSet)
	d, err := mergePatch.Data(desiredStatefulSet)
	GinkgoWriter.Printf("the patch is: %v. \n", string(d))
	if err != nil {
		return fmt.Errorf("failed to generate patch, err is %w", err)
	}

	return frame.PatchResource(desiredStatefulSet, mergePatch, opts...)
}

func RestartAndValidateStatefulSetPodIP(frame *e2e.Framework, label map[string]string) error {
	stsPodList, err := frame.GetPodListByLabel(label)
	if err != nil {
		return err
	}

	if len(stsPodList.Items) == 0 {
		return nil
	}

	oldIPList, err := recordStatefulSetPodIP(stsPodList)
	if err != nil {
		return err
	}
	GinkgoWriter.Printf("statefulset old IP list %v \n", oldIPList)

	if err := frame.DeletePodList(stsPodList); err != nil {
		GinkgoWriter.Printf("statefulset old IP list %v \n", oldIPList)
	}

	newStsPodList, err := frame.GetPodListByLabel(label)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), PodReStartTimeout)
	defer cancel()
	err = frame.WaitPodListRunning(label, len(stsPodList.Items), ctx)
	if err != nil {
		return err
	}

	newIPList, err := recordStatefulSetPodIP(newStsPodList)
	if err != nil {
		return err
	}
	GinkgoWriter.Printf("statefulset new IP list %v \n", newIPList)

	if len(oldIPList) != len(newIPList) {
		return fmt.Errorf("oldIPList and newIPList have different lengths: %d vs %d", len(oldIPList), len(newIPList))
	}

	for key, oldValue := range oldIPList {
		if newValue, ok := newIPList[key]; !ok || newValue != oldValue {
			return fmt.Errorf("oldIPList and newIPList differ at key %s: old value = %v, new value = %v", key, oldIPList, newIPList)
		}
	}

	return nil
}

func recordStatefulSetPodIP(podList *corev1.PodList) (map[string]string, error) {
	recordIPMap := make(map[string]string)
	for _, pod := range podList.Items {
		for _, ip := range pod.Status.PodIPs {
			ipStr := ip.IP
			if existingPod, ok := recordIPMap[ipStr]; ok {
				return nil, fmt.Errorf("the IP address: %v of Pod %v conflicts with the IP address: %v of Pod %v", ipStr, existingPod, ipStr, pod.Name)
			} else {
				recordIPMap[ipStr] = pod.Name
			}
		}
	}
	return recordIPMap, nil
}
