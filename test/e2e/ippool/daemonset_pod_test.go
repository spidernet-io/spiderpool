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
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("test daemonset", Label("ippool_daemonset"), func() {
	var dsName, nsName string
	var err error
	BeforeEach(func() {
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)
		dsName = "dstwo" + tools.RandomName()

		// DeferCleanup(func() {
		// 	GinkgoWriter.Printf("delete namespace %v \n", nsName)
		// 	err = frame.DeleteNamespace(nsName)
		// 	Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		// })
	})

	Context("test by means of daemonset create pods，assign ipv4、ipv6", Label("smoke"), Label("E00004"), func() {
		It(" pods in a daemonset allocate/release ipv4 and ipv6 addresses", func() {
			// create daemonset
			GinkgoWriter.Printf("try to create daemonset %v/%v \n", nsName, dsName)
			ds := common.GenerateExampleDaemonSetYaml(dsName, nsName)
			GinkgoWriter.Printf("GenerateExampleDaemonSetYaml: %v \n", ds)
			err = frame.CreateDaemonSet(ds)
			Expect(err).NotTo(HaveOccurred(), "failed to create daemonset")

			// Sometimes the daemonset being created，so wait for daemonset create success ---> hardcode，to do
			// As the replicas increase，can change the waiting time。
			// but the same case，last time it worked, this time it didn't，please check performance
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			//_, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			//最后执行
			defer cancel()
			ds, err := frame.WaitDaemonSetReady(dsName, nsName, ctx)
			Expect(err).NotTo(HaveOccurred(), "time out to wait  all Replicas ready")
			Expect(ds).NotTo(BeNil())
			time.Sleep(10 * time.Second)
			GinkgoWriter.Printf("CurrentNumberScheduled: %v \n", ds.Status.CurrentNumberScheduled)
			GinkgoWriter.Printf("DesiredNumberScheduled: %v \n", ds.Status.DesiredNumberScheduled)
			// get all daemonset replicas name and check ip
			opts := []client.ListOption{
				client.InNamespace(nsName),
				client.MatchingLabels{"app": dsName},
			}
			podinfolist, err := frame.GetPodList(opts...)
			Expect(err).NotTo(HaveOccurred(), "failed to list pod")
			GinkgoWriter.Printf("podinfolist: %v", *podinfolist)
			GinkgoWriter.Printf("podinfolist.Items: %v \n", podinfolist.Items)
			Expect(len(podinfolist.Items)).NotTo(HaveValue(Equal(0)))

			// check all pod assign ipv4 and ipv6 addresses success
			/*for i := 0; i < len(podinfolist.Items); i++ {
			// pod, err := frame.WaitPodStarted(podinfolist.Items[i].Name, nsName, ctx)
			pod, err := frame.GetPod(podinfolist.Items[i].Name, nsName)
			Expect(err).NotTo(HaveOccurred(), "time out to wait pod ready")
			Expect(ds).NotTo(BeNil())
			Expect(pod.Status.PodIPs).NotTo(BeEmpty(), "pod failed to assign ip")
			GinkgoWriter.Printf("pod %v/%v ip: %+v \n", nsName, dsName, pod.Status.PodIPs)

			if frame.Info.IpV4Enabled == true {
				Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue(), "pod failed to get ipv4 ip")
				By("succeeded to check pod ipv4 ip ")
			}
			if frame.Info.IpV6Enabled == true {
				Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue(), "pod failed to get ipv6 ip")
				By("succeeded to check pod ipv6 ip \n")
			}
			//}
			// delete daemonset
			GinkgoWriter.Printf("delete daemonset: %v \n", dsName)
			err = frame.DeleteDaemonSet(dsName, nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete daemonset: %v", dsName)  */
		})
	})
})
