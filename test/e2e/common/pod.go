// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	"github.com/spidernet-io/e2eframework/tools"
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
