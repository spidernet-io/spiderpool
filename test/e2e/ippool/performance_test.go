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

var _ = Describe("test performance", Label("performance"), func() {
	var dpmName, nsName string
	var err error
	BeforeEach(func() {
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)
		dpmName = "dpm" + tools.RandomName()
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})
	Context("through controller deployment CRUD，check time cost for assigning ipv4、ipv6 to 100 pods",
		Label("performance"), Label("P00002"), func() {
			It("time cost for assigning ipv4、ipv6 to 100 pods", func() {
				// create deployment，record the creation time
				startT := time.Now()
				GinkgoWriter.Printf("try to create deployment %v/%v/%v \n", dpmName, nsName)
				dpm := common.GenerateExampleDeploymentYaml(dpmName, nsName, 100)
				err = frame.CreateDeployment(dpm)
				Expect(err).NotTo(HaveOccurred(), "failed to create deployment")

				// As the replicas increase，can change the waiting time。
				// but the same case，last time it worked, this time it didn't，please check performance
				ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
				defer cancel()
				dpm, err := frame.WaitDeploymentReady(dpmName, "", ctx)
				Expect(err).NotTo(HaveOccurred(), "time out to wait")
				Expect(dpm).NotTo(BeNil())

				// computing time
				endT := time.Since(startT)
				GinkgoWriter.Printf("time cost for create deployment replicas to 100 = %v\n", endT)

				// time cost for update deployment replicas to 50
				startT = time.Now()
				*(dpm.Spec.Replicas) = 50
				err = frame.UpdateDeployment(dpm)
				Expect(err).NotTo(HaveOccurred(), "failed to update deployment")

				dpm, err = frame.WaitDeploymentReady(dpmName, nsName, ctx)
				Expect(err).NotTo(HaveOccurred(), "time out to wait  all Replicas ready")
				Expect(dpm).NotTo(BeNil())

				// computing time
				endT = time.Since(startT)
				GinkgoWriter.Printf("time cost for update deployment replicas to 50 = %v\n", endT)

				// get all deployment replicas name and check ip
				opts := []client.ListOption{
					client.InNamespace(nsName),
					client.MatchingLabels{"app": dpmName},
				}
				podinfolist, err := frame.GetPodList(opts...)
				Expect(err).NotTo(HaveOccurred(), "failed to list pod")
				Expect(len(podinfolist.Items)).NotTo(HaveValue(Equal(0)))

				// check all pod  assign ipv4 and ipv6 addresses success
				for i := 0; i < len(podinfolist.Items); i++ {
					// pod, err := frame.WaitPodStarted(podinfolist.Items[i].Name, nsName, ctx)
					pod, err := frame.GetPod(podinfolist.Items[i].Name, nsName)
					Expect(err).NotTo(HaveOccurred(), "failed to get pod information")
					Expect(pod).NotTo(BeNil())
					Expect(pod.Status.PodIPs).NotTo(BeEmpty(), "pod failed to assign ip")
					GinkgoWriter.Printf("pod %v/%v ip: %+v \n", nsName, dpmName, pod.Status.PodIPs)

					if frame.Info.IpV4Enabled == true {
						Expect(tools.CheckPodIpv4IPReady(pod)).To(BeTrue(), "pod failed to get ipv4 ip")
						By("succeeded to check pod ipv4 ip ")
					}
					if frame.Info.IpV6Enabled == true {
						Expect(tools.CheckPodIpv6IPReady(pod)).To(BeTrue(), "pod failed to get ipv6 ip")
						By("succeeded to check pod ipv6 ip \n")
					}
				}

				// time cost for delete deployment replicas to 50
				startT = time.Now()
				GinkgoWriter.Printf("delete deployment: %v \n", dpmName)
				err = frame.DeleteDeployment(dpmName, nsName)
				Expect(err).NotTo(HaveOccurred(), "failed to delete deployment: %v", dpmName)

				for {
					podinfolist, err := frame.GetPodList(opts...)
					Expect(err).NotTo(HaveOccurred(), "failed to list pod")
					if len(podinfolist.Items) == 0 {
						break
					}
					// wait for 1s, avoid frequent requests
					time.Sleep(time.Duration(1) * time.Second)
				}
				// computing time
				endT = time.Since(startT)
				GinkgoWriter.Printf("time cost for delete deployment replicas to 50 = %v\n", endT)
			})
		})
})