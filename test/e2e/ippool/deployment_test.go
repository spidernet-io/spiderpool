// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippool_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test ip with deployment case", Label("deployment"), func() {

	var dpmName, nsName string
	var err error

	BeforeEach(func() {

		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// init deployment name
		dpmName = "dpm" + tools.RandomName()

		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	It("Two pods in a deployment allocate/release ipv4 and ipv6 addresses", Label("smoke", "E00002"), func() {

		// create deployment
		GinkgoWriter.Printf("try to create deployment %v/%v \n", dpmName, nsName)
		dpm := common.GenerateExampleDeploymentYaml(dpmName, nsName, 2)
		err = frame.CreateDeployment(dpm)
		Expect(err).NotTo(HaveOccurred(), "failed to create deployment")

		// waiting for deployment replicas to complete
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		dpm, err := frame.WaitDeploymentReady(dpmName, nsName, ctx)
		Expect(err).NotTo(HaveOccurred(), "time out to wait all Replicas ready")
		Expect(dpm).NotTo(BeNil())

		// check two pods created by deployment，its assign ipv4 and ipv6 addresses success
		podlist, err := frame.GetDeploymentPodList(dpm)
		Expect(err).NotTo(HaveOccurred(), "failed to list pod")
		Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))

		for i := 0; i < len(podlist.Items); i++ {
			Expect(podlist.Items[i].Status.PodIPs).NotTo(BeEmpty(), "pod %v failed to assign ip", podlist.Items[i].Name)
			GinkgoWriter.Printf("pod %v/%v ips: %+v \n", nsName, podlist.Items[i].Name, podlist.Items[i].Status.PodIPs)

			if frame.Info.IpV4Enabled == true {
				Expect(tools.CheckPodIpv4IPReady(&podlist.Items[i])).To(BeTrue(), "pod %v failed to get ipv4 ip", podlist.Items[i].Name)
				By("succeeded to check pod ipv4 ip ")
			}
			if frame.Info.IpV6Enabled == true {
				Expect(tools.CheckPodIpv6IPReady(&podlist.Items[i])).To(BeTrue(), "pod %v failed to get ipv6 ip", podlist.Items[i].Name)
				By("succeeded to check pod ipv6 ip \n")
			}
		}

		// delete deployment
		GinkgoWriter.Printf("delete deployment: %v \n", dpmName)
		err = frame.DeleteDeployment(dpmName, nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to delete deployment: %v \n", dpmName)
	})
})
