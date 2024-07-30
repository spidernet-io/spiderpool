// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"encoding/json"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateExamplePodYaml(podName, namespace string) *corev1.Pod {
	Expect(podName).NotTo(BeEmpty(), "podName is a empty string")
	Expect(namespace).NotTo(BeEmpty(), "namespace is a empty string")

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:   namespace,
			Name:        podName,
			Annotations: map[string]string{},
			Labels: map[string]string{
				podName: podName,
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
	}
}

func CreatePodUntilReady(frame *e2e.Framework, podYaml *corev1.Pod, podName, namespace string, waitPodStartTimeout time.Duration) (pod *corev1.Pod, podIPv4, podIPv6 string) {
	// create pod
	GinkgoWriter.Printf("create pod %v/%v \n", namespace, podName)
	err := frame.CreatePod(podYaml)
	Expect(err).NotTo(HaveOccurred(), "failed to create pod")

	// wait for pod ip
	GinkgoWriter.Printf("wait for pod %v/%v ready \n", namespace, podName)
	ctx, cancel := context.WithTimeout(context.Background(), waitPodStartTimeout)

	defer cancel()
	pod, err = frame.WaitPodStarted(podName, namespace, ctx)
	Expect(err).NotTo(HaveOccurred(), "time out to wait pod ready")
	Expect(pod).NotTo(BeNil(), "pod is nil")
	Expect(pod.Status.PodIPs).NotTo(BeEmpty(), "pod failed to assign ip")

	GinkgoWriter.Printf("pod: %v/%v, ips: %+v \n", namespace, podName, pod.Status.PodIPs)

	var ok bool
	if frame.Info.IpV4Enabled {
		podIPv4, ok = tools.CheckPodIpv4IPReady(pod)
		Expect(ok).NotTo(BeFalse(), "failed to get ipv4 ip")
		Expect(podIPv4).NotTo(BeEmpty(), "podIPv4 is a empty string")
		GinkgoWriter.Println("succeeded to check pod ipv4 ip")
	}
	if frame.Info.IpV6Enabled {
		podIPv6, ok = tools.CheckPodIpv6IPReady(pod)
		Expect(ok).NotTo(BeFalse(), "failed to get ipv6 ip")
		Expect(podIPv6).NotTo(BeEmpty(), "podIPv6 is a empty string")
		GinkgoWriter.Println("succeeded to check pod ipv6 ip")
	}
	return
}

func CreatePodWithAnnoPodIPPool(frame *e2e.Framework, podName, namespace string, annoPodIPPoolValue types.AnnoPodIPPoolValue) {
	Expect(podName).NotTo(BeEmpty(), "podName is empty\n")
	Expect(namespace).NotTo(BeEmpty(), "namespace is empty\n")

	GinkgoWriter.Printf("marshal annoPodIPPoolValue: %+v\n", annoPodIPPoolValue)
	b, err := json.Marshal(annoPodIPPoolValue)
	Expect(err).NotTo(HaveOccurred(), "failed to marshal annoPodIPPoolValue\n")
	annoPodIPPoolValueStr := string(b)

	GinkgoWriter.Printf("generate pod %v/%v yaml \n", namespace, podName)
	podYaml := GenerateExamplePodYaml(podName, namespace)
	Expect(podYaml).NotTo(BeNil(), "failed to generate pod yaml")
	podYaml.Annotations = map[string]string{
		constant.AnnoPodIPPool: annoPodIPPoolValueStr,
	}

	GinkgoWriter.Printf("create pod %v/%v\n", namespace, podName)
	Expect(frame.CreatePod(podYaml)).To(Succeed(), "failed to create pod %v/%v\n", namespace, podName)
	ctx, cancel := context.WithTimeout(context.Background(), PodStartTimeout)
	defer cancel()
	pod, err := frame.WaitPodStarted(podName, namespace, ctx)
	Expect(err).NotTo(HaveOccurred(), "failed to wait pod %v/%v started\n", namespace, podName)
	GinkgoWriter.Printf("pod %v/%v anno: %+v\n", namespace, podName, pod.Annotations)
}

func CheckPodIpReadyByLabel(frame *e2e.Framework, label map[string]string, v4PoolNameList, v6PoolNameList []string) *corev1.PodList {

	Expect(label).NotTo(BeNil(), "label is nil \n")
	Expect(frame).NotTo(BeNil(), "frame is nil \n")

	// Get the rebuild pod list
	podList, err := frame.GetPodListByLabel(label)
	Expect(err).NotTo(HaveOccurred(), "Failed to get pod list, %v \n", err)
	Expect(len(podList.Items)).NotTo(Equal(0))

	// Succeeded to assign ipv4、ipv6 ip for pod
	Expect(frame.CheckPodListIpReady(podList)).NotTo(HaveOccurred(), "failed to check ipv4 or ipv6 ,reason=%v \n", err)

	// check pod ip recorded in ippool
	ok, _, _, e := CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
	Expect(e).NotTo(HaveOccurred(), "Failed to check Pod IP Record In IPPool, error is %v \n", err)
	Expect(ok).To(BeTrue())
	GinkgoWriter.Printf("Pod IP recorded in IPPool %v , %v \n", v4PoolNameList, v6PoolNameList)
	return podList
}

func DeletePods(frame *e2e.Framework, opts ...client.DeleteAllOfOption) error {
	return frame.KClient.DeleteAllOf(context.TODO(), &corev1.Pod{}, opts...)
}
