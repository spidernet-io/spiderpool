// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"time"

	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenerateExamplePodYaml(podName, namespace string) *corev1.Pod {
	return GenerateLongPodYaml(podName, namespace, 0)
}

func GenerateLongPodYaml(podName, namespace string, annotationLength int) *corev1.Pod {
	Expect(podName).NotTo(BeEmpty(), "podName is a empty string")
	Expect(namespace).NotTo(BeEmpty(), "namespace is a empty string")

	annotationStr := GenerateString(annotationLength)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      podName,
			Annotations: map[string]string{
				"test": annotationStr,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "samplepod",
					Image:           "alpine",
					ImagePullPolicy: "IfNotPresent",
					Command:         []string{"/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"},
				},
			},
		},
	}
}

func CreatePodUntilReady(frame *e2e.Framework, podYaml *corev1.Pod, podName, namespace string) (pod *corev1.Pod, podIPv4, podIPv6 string) {
	// create pod
	GinkgoWriter.Printf("create pod %v/%v \n", namespace, podName)
	err := frame.CreatePod(podYaml)
	Expect(err).NotTo(HaveOccurred(), "failed to create pod")

	// wait for pod ip
	GinkgoWriter.Printf("wait for pod %v/%v ready \n", namespace, podName)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pod, err = frame.WaitPodStarted(podName, namespace, ctx)
	Expect(err).NotTo(HaveOccurred(), "time out to wait pod ready")
	Expect(pod).NotTo(BeNil(), "pod is nil")
	Expect(pod.Status.PodIPs).NotTo(BeEmpty(), "pod failed to assign ip")

	GinkgoWriter.Printf("pod: %v/%v, ips: %+v \n", namespace, podName, pod.Status.PodIPs)

	if frame.Info.IpV4Enabled {
		Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue(), "pod failed to get ipv4 ip")
		GinkgoWriter.Println("succeeded to check pod ipv4 ip")
		// get ipv4
		GinkgoWriter.Println("get IPv4")
		podIPv4 = GetPodIPv4(pod)
		GinkgoWriter.Printf("pod IPv4: %+v \n", podIPv4)
		Expect(podIPv4).NotTo(BeEmpty(), "podIPv4 is a empty string")
	}
	if frame.Info.IpV6Enabled {
		Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue(), "pod failed to get ipv6 ip")
		GinkgoWriter.Println("succeeded to check pod ipv6 ip")
		// get ipv6
		GinkgoWriter.Println("get IPv6")
		podIPv6 = GetPodIPv6(pod)
		GinkgoWriter.Printf("pod IPv6: %+v\n", podIPv6)
		Expect(podIPv6).NotTo(BeEmpty(), "podIPv6 is a empty string")
	}
	return
}

func GetPodIPv4(pod *corev1.Pod) string {
	podIPs := pod.Status.PodIPs
	for _, v := range podIPs {
		if govalidator.IsIPv4(v.IP) {
			return v.IP
		}
	}
	return ""
}

func GetPodIPv6(pod *corev1.Pod) string {
	podIPs := pod.Status.PodIPs
	for _, v := range podIPs {
		if govalidator.IsIPv6(v.IP) {
			return v.IP
		}
	}
	return ""
}
