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

	// Resolve the owner StatefulSet to get the desired replicas, so that the
	// pod list is captured only after the StatefulSet is stable. Otherwise a
	// pod that has not been created or has not been assigned an IP yet (e.g.
	// right after a scale-up under IPPool contention) would be silently
	// missed, causing a false mismatch between the old and new IP lists.
	stsName, stsNamespace := "", ""
	for _, ownerRef := range stsPodList.Items[0].OwnerReferences {
		if ownerRef.Kind == "StatefulSet" {
			stsName = ownerRef.Name
			stsNamespace = stsPodList.Items[0].Namespace
			break
		}
	}
	if stsName == "" {
		return fmt.Errorf("failed to find the owner StatefulSet of pod %s/%s", stsPodList.Items[0].Namespace, stsPodList.Items[0].Name)
	}
	sts, err := frame.GetStatefulSet(stsName, stsNamespace)
	if err != nil {
		return fmt.Errorf("failed to get StatefulSet %s/%s, error %w", stsNamespace, stsName, err)
	}
	expectedReplicas := int(*sts.Spec.Replicas)

	ctx, cancel := context.WithTimeout(context.Background(), PodReStartTimeout*2)
	defer cancel()

	stsPodList, err = waitStatefulSetPodListReadyWithIP(ctx, frame, label, expectedReplicas)
	if err != nil {
		return fmt.Errorf("failed to wait for StatefulSet %s/%s pods to be ready with IP before restart, error %w", stsNamespace, stsName, err)
	}

	oldIPList, err := recordStatefulSetPodIP(stsPodList)
	if err != nil {
		return err
	}
	GinkgoWriter.Printf("statefulset old IP list %v \n", oldIPList)

	if err := frame.DeletePodList(stsPodList); err != nil {
		GinkgoWriter.Printf("statefulset old IP list %v \n", oldIPList)
	}

	newStsPodList, err := waitStatefulSetPodListReadyWithIP(ctx, frame, label, expectedReplicas)
	if err != nil {
		return fmt.Errorf("failed to wait for StatefulSet %s/%s pods to be ready with IP after restart, error %w", stsNamespace, stsName, err)
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

// waitStatefulSetPodListReadyWithIP waits until the number of pods matching the
// label equals the expected replicas, all pods are running, and every pod has
// been assigned at least one IP address, then returns the pod list.
func waitStatefulSetPodListReadyWithIP(ctx context.Context, frame *e2e.Framework, label map[string]string, expectedReplicas int) (*corev1.PodList, error) {
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timed out waiting for %d running pods with IP for label %v", expectedReplicas, label)
		default:
			podList, err := frame.GetPodListByLabel(label)
			if err != nil {
				return nil, err
			}
			if len(podList.Items) == expectedReplicas && frame.CheckPodListRunning(podList) {
				allPodsHaveIP := true
				for _, pod := range podList.Items {
					if len(pod.Status.PodIPs) == 0 {
						allPodsHaveIP = false
						break
					}
				}
				if allPodsHaveIP {
					return podList, nil
				}
			}
		}
		time.Sleep(ForcedWaitingTime)
	}
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
