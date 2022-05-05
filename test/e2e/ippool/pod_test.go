// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippool_test

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	// corev1 "k8s.io/api/core/v1"
	// metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/spidernet-io/e2eframework/tools"
	"time"
)

var _ = Describe("test pod", Label("ippool_pod"), func() {
	var err error
	var podName, namespace string

	BeforeEach(func() {
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespace(namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
		podName = "pod" + tools.RandomName()

		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		})
	})

	Context("test default ippool", Label("smoke"), Label("E00001"), func() {
		It("", func() {
			// create pod
			GinkgoWriter.Printf("try to create pod %v/%v \n", namespace, podName)
			pod := common.GenerateExamplePodYaml(podName, namespace)

			err = frame.CreatePod(pod)
			Expect(err).NotTo(HaveOccurred(), "failed to create pod")

			// wait for pod ip
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			pod, err = frame.WaitPodStarted(podName, namespace, ctx)
			Expect(err).NotTo(HaveOccurred(), "time out to wait pod ready")
			Expect(pod).NotTo(BeNil())

			// check pod ip
			// pod, err := frame.GetPod(podName, namespace )
			// Expect(err).NotTo(HaveOccurred())
			Expect(pod.Status.PodIPs).NotTo(BeEmpty(), "pod failed to assign ip")

			GinkgoWriter.Printf("pod %v/%v ip: %+v \n", namespace, podName, pod.Status.PodIPs)
			if frame.Info.IpV4Enabled == true {
				Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue(), "pod failed to get ipv4 ip")
				By("succeeded to check pod ipv4 ip ")
			}
			if frame.Info.IpV6Enabled == true {
				Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue(), "pod failed to get ipv6 ip")
				By("succeeded to check pod ipv6 ip \n")
			}

			// delete pod
			err = frame.DeletePod(podName, namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete pod")
		})
	})

})
