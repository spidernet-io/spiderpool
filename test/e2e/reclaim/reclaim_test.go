// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reclaim_test

import (
	"time"

	"golang.org/x/net/context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test ip with reclaim ip case", Label("reclaim"), func() {
	var err error
	var podName, namespace, podIPv4, podIPv6 string

	BeforeEach(func() {
		// create namespace
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err = frame.CreateNamespace(namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)

		// pod name
		podName = "pod" + tools.RandomName()

		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		})
	})

	It("related IP resource recorded in ippool will be reclaimed after the namespace is deleted",
		Label("smoke", "G00001"), func() {
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

	Context("delete the same-name pod within a different namespace", func() {
		// declare another namespace namespace1
		var namespace1 string

		BeforeEach(func() {
			// create namespace1
			namespace1 = "ns1-" + tools.RandomName()
			GinkgoWriter.Printf("create namespace1 %v \n", namespace1)
			err = frame.CreateNamespace(namespace1)
			Expect(err).NotTo(HaveOccurred(), "failed to create namespace1 %v", namespace1)

			DeferCleanup(func() {
				GinkgoWriter.Printf("delete namespace1 %v \n", namespace1)
				err := frame.DeleteNamespace(namespace1)
				Expect(err).NotTo(HaveOccurred(), "failed to delete namespace1 %v", namespace1)
			})
		})

		It("the IP of a running pod should not be reclaimed after a same-name pod within a different namespace is deleted",
			Label("G00002"), func() {

				namespaces := []string{namespace, namespace1}
				for _, ns := range namespaces {
					// create pod in namespace
					GinkgoWriter.Println("generate example pod yaml")
					pod := common.GenerateExamplePodYaml(podName, ns)
					GinkgoWriter.Printf("succeed generate pod yaml: %v\n", pod)
					Expect(pod).NotTo(BeNil())
					GinkgoWriter.Printf("create pod %v/%v\n", ns, podName)
					pod, _, _ = common.CreatePodUntilReady(frame, pod, podName, ns, time.Second*30)
					Expect(pod).NotTo(BeNil())
					GinkgoWriter.Printf("succeed create pod: %v/%v\n", ns, podName)
				}

				// delete pod in namespace until finish
				GinkgoWriter.Printf("delete the pod %v in namespace1 %v\n", podName, namespace1)
				ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
				defer cancel1()
				e2 := frame.DeletePodUntilFinish(podName, namespace1, ctx1)
				Expect(e2).NotTo(HaveOccurred())
				GinkgoWriter.Printf("succeed delete pod %v/%v\n", namespace1, podName)

				// check if pod in namespace is running normally
				GinkgoWriter.Printf("check if pod %v in namespace %v is running normally\n", podName, namespace)
				pod3, e3 := frame.GetPod(podName, namespace)
				Expect(pod3).NotTo(BeNil())
				Expect(e3).NotTo(HaveOccurred())
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Println("check pod ipv4")
					podIPv4 = common.GetPodIPv4(pod3)
					Expect(podIPv4).NotTo(BeEmpty())
					GinkgoWriter.Printf("pod ipv4: %v\n", podIPv4)
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Println("check pod ipv6")
					podIPv6 = common.GetPodIPv6(pod3)
					Expect(podIPv6).NotTo(BeEmpty())
					GinkgoWriter.Printf("pod ipv6: %v\n", podIPv6)
				}

				// TODO(bingzhesun) check the same-name pod , its ip in ippool not be reclaimed
			})
	})
})
