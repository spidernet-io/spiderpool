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

var _ = Describe("test ip with namespace case", Label("ippool_namespace"), func() {

	var err error
	var podName, namespace string

	BeforeEach(func() {
		// create namespace
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err = frame.CreateNamespace(namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)

		// pod name
		podName = "pod" + tools.RandomName()
	})

	It("ip should be gc after namespace is deleted", Label("smoke", "E00008"), func() {
		// generate podYaml
		podYaml := common.GenerateExamplePodYaml(podName, namespace)
		Expect(podYaml).NotTo(BeNil())
		GinkgoWriter.Printf("podYaml: %v \n", podYaml)

		// create pod
		pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, namespace, time.Second*30)
		Expect(pod).NotTo(BeNil(), "create pod failed")

		// ippool allocated ip
		var allocatedIPv4s, allocatedIPv6s []string

		// get ippool status.allocated_ips
		// TODO(bingzhesun) getAllocatedIPs() return allocatedIPv4s and allocatedIPv6s

		if podIPv4 != "" {
			// TODO(bingzhesun) here we assume that we have obtained the allocated ips
			allocatedIPv4s = append(allocatedIPv4s, podIPv4)
			GinkgoWriter.Printf("allocatedIPv4s: %v\n", allocatedIPv4s)

			// check if podIP in ippool
			GinkgoWriter.Println("check if podIPv4 in ippool")
			Expect(allocatedIPv4s).To(ContainElement(podIPv4), "assign ipv4 failed")
		}
		if podIPv6 != "" {
			// TODO(bingzhesun) here we assume that we have obtained the allocated ips
			allocatedIPv6s = append(allocatedIPv6s, podIPv6)
			GinkgoWriter.Printf("allocatedIPv6s: %v\n", allocatedIPv6s)

			// check if podIP in ippool
			GinkgoWriter.Println("check if podIPv6 in ippool")
			Expect(allocatedIPv6s).To(ContainElement(podIPv6), "assign ipv6 failed")
		}

		// delete namespace
		GinkgoWriter.Printf("delete namespace %v\n", namespace)
		err = frame.DeleteNamespace(namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		// TODO(bingzhesun) Here we will use the function waitNamespaceDeleted() to judge

		// get ippool status.allocated_ips after delete namespace
		// TODO(bingzhesun) getAllocatedIPs() return allocatedIPv4s and allocatedIPv6s
		// here we assume that we have obtained the allocated ips

		//  TODO(bingzhesun) check if podIP in ippool

	})
})
