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

var _ = Describe("test ip with daemonset case", Label("daemonset"), func() {

	var dsName, nsName string
	var err error

	BeforeEach(func() {

		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// init daemonset name
		dsName = "ds" + tools.RandomName()

		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	It("one daemonset allocate/release ipv4 and ipv6 addresses", Label("smoke", "E00004"), func() {

		// create daemonset
		GinkgoWriter.Printf("try to create daemonset %v/%v \n", dsName, nsName)
		ds := common.GenerateExampleDaemonSetYaml(dsName, nsName)
		err = frame.CreateDaemonSet(ds)
		Expect(err).NotTo(HaveOccurred(), "failed to create daemontset")

		// waiting for daemontset replicas to complete
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		ds, err := frame.WaitDaemonSetReady(dsName, nsName, ctx)
		Expect(err).NotTo(HaveOccurred(), "time out to wait all Replicas ready")
		Expect(ds).NotTo(BeNil())

		// check  pods created by daemontset，its assign ipv4 and ipv6 addresses success
		podlist, err := frame.GetDaemonSetPodList(ds)
		Expect(err).NotTo(HaveOccurred(), "failed to list pod")
		Expect(int32(len(podlist.Items))).Should(Equal(ds.Status.NumberReady))

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

		// delete daemontset
		GinkgoWriter.Printf("delete daemontset: %v \n", dsName)
		err = frame.DeleteDaemonSet(dsName, nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to delete daemontset: %v \n", dsName)
	})
})
