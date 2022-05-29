// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippool_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
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

	DescribeTable("test default ippool", func(annotationLength int) {
		// create pod
		GinkgoWriter.Printf("create pod %v/%v with annotationLength= %v \n", namespace, podName, annotationLength)
		podYaml := common.GenerateLongPodYaml(podName, namespace, annotationLength)
		Expect(podYaml).NotTo(BeNil())
		pod, _, _ := common.CreatePodUntilReady(frame, podYaml, podName, namespace, time.Second*30)
		Expect(pod).NotTo(BeNil())
		Expect(pod.Annotations["test"]).To(Equal(podYaml.Annotations["test"]))
		GinkgoWriter.Printf("create pod %v/%v successfully \n", namespace, podName)
		// delete pod
		GinkgoWriter.Printf("delete pod %v/%v \n", namespace, podName)
		err = frame.DeletePod(podName, namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", namespace, podName)
	},
		Entry("test normal pod until it is ready", Label("smoke", "E00001"), 0),
		Entry("test longYaml pod until it is ready", Label("smoke", "E00009"), 100),
	)
})
