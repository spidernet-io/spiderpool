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

var _ = Describe("Delete namespace with pod, check ip gc", Label("ippool_namespace", "smoke"), func() {

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

	It("ip should be gc after namespace is deleted", Label("E00008"), func() {

		// create pod
		pod, podIPv4, podIPv6 := common.CreatePod(frame, common.GenerateExamplePodYaml, podName, namespace)
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
		time.Sleep(time.Second * 10)

		// get ippool status.allocated_ips after delete namespace
		// TODO(bingzhesun) getAllocatedIPs() return allocatedIPv4s and allocatedIPv6s
		// here we assume that we have obtained the allocated ips
		allocatedIPv4s = allocatedIPv4s[0:0]
		allocatedIPv6s = allocatedIPv6s[0:0]
		GinkgoWriter.Printf("allocatedIPv4s: %v\n", allocatedIPv4s)
		GinkgoWriter.Printf("allocatedIPv6s: %v\n", allocatedIPv6s)

		// check if podIP in ippool
		GinkgoWriter.Println("check if podIP in ippool")
		Expect(allocatedIPv4s).NotTo(ContainElement(podIPv4), "release ipv4 failed")
		Expect(allocatedIPv6s).NotTo(ContainElement(podIPv6), "release ipv6 failed")
	})
})
