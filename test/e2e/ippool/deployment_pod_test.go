// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippool_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("test pod", Label("ippool_pod"), func() {
	var deploymentName, podName, nsName string
	var err error
	BeforeEach(func() {
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)
		podName = "pod" + tools.RandomName()
		deploymentName = "dep" + tools.RandomName()

		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	Context("test by means of deployment create pods，assign ipv4、ipv6", Label("smoke"), Label("E00002"), func() {
		It("Two pods in a deployment allocate/release ipv4 and ipv6 addresses", func() {
			// create deployment
			GinkgoWriter.Printf("try to create deployment %v/%v/%v \n", deploymentName, nsName, podName)
			err = frame.CreateDeployment(deploymentName, podName, nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to create deployment pod")
			// Sometimes the deployment being created，so wait for deployment create success ---> hardcode，to do
			time.Sleep(time.Duration(20) * time.Second)
			podinfolist, err := frame.GetPodList(&client.ListOptions{Namespace: nsName})
			// podinfolist, err := frame.GetPodList(&client.ListOptions{LabelSelector: })
			Expect(err).NotTo(HaveOccurred(), "failed to list pod")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// check all pod  assign ipv4 and ipv6 addresses
			for i := 0; i < len(podinfolist.Items); i++ {
				GinkgoWriter.Printf("pod name: %v  \n", podinfolist.Items[i].Name)
				pod, err := frame.WaitPodStarted(podinfolist.Items[i].Name, nsName, ctx)
				Expect(err).NotTo(HaveOccurred(), "time out to wait pod ready")
				Expect(pod).NotTo(BeNil())
				Expect(pod.Status.PodIPs).NotTo(BeEmpty(), "pod failed to assign ip")
				GinkgoWriter.Printf("pod %v/%v ip: %+v \n", nsName, deploymentName, pod.Status.PodIPs)
				if frame.Info.IpV4Enabled == true {
					Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue(), "pod failed to get ipv4 ip")
					By("succeeded to check pod ipv4 ip ")
				}
				if frame.Info.IpV6Enabled == true {
					Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue(), "pod failed to get ipv6 ip")
					By("succeeded to check pod ipv6 ip \n")
				}
			}
			// delete deployment
			GinkgoWriter.Printf("delete deployment： %v \n", deploymentName)
			err = frame.DeleteDeployment(deploymentName, nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete deployment： %v", deploymentName)
		})
	})
})
