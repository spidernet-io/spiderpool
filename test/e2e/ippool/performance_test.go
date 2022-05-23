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

var _ = Describe("about ippool performance test case", Label("performance"), func() {
	var perName, nsName string
	var err error
	var creReplicas, scaReplicas int32
	BeforeEach(func() {
		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// init ippool performance test name
		perName = "per" + tools.RandomName()

		// init CRUD replicas info
		creReplicas = 50
		scaReplicas = 100

		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	Context("through controller deployment CRUD，check time cost for assigning ipv4、ipv6 to 100 pods", func() {
		// TODO (tao.yang), the kind cluster could not create more pods
		// future improvement remove master taint or add node
		It("time cost for assigning ipv4、ipv6 to 100 pods", Label("performance"), Label("P00002"), func() {

			// create deployment，record the creation time
			GinkgoWriter.Printf("try to create deployment %v/%v \n", perName, nsName)
			dpm := common.GenerateExampleDeploymentYaml(perName, nsName, creReplicas)
			err = frame.CreateDeployment(dpm)
			Expect(err).NotTo(HaveOccurred(), "failed to create deployment")

			// As the replicas increase，can change the waiting time。
			// but the same case，last time it worked, this time it didn't，please check performance
			ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
			defer cancel()
			// computing create deployment time cost
			startT1 := time.Now()
			dpm, err := frame.WaitDeploymentReady(perName, nsName, ctx)
			Expect(err).NotTo(HaveOccurred(), "time out to wait deployment ready")
			Expect(dpm).NotTo(BeNil())
			endT1 := time.Since(startT1)
			GinkgoWriter.Printf("time cost for create deployment of %v replicas= %v \n", creReplicas, endT1)

			dpm, err = frame.ScaleDeployment(dpm, scaReplicas)
			Expect(err).NotTo(HaveOccurred(), "failed to scale deployment replicas")
			Expect(dpm).NotTo(BeNil())

			// time cost for scale deployment replicas
			startT2 := time.Now()
			dpm, err = frame.WaitDeploymentReady(perName, nsName, ctx)
			Expect(err).NotTo(HaveOccurred(), "time out to wait deployment replicas ready")
			Expect(dpm).NotTo(BeNil())
			endT2 := time.Since(startT2)
			GinkgoWriter.Printf("time cost for scale deployment of %v replicas = %v \n", scaReplicas, endT2)

			// get deployment pod list
			podlist, err := frame.GetDeploymentPodList(dpm)
			Expect(err).NotTo(HaveOccurred(), "failed to list pod")
			Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))

			// check all pods created by deployment，its assign ipv4 and ipv6 addresses success
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

			GinkgoWriter.Printf("delete deployment: %v \n", perName)
			err = frame.DeleteDeployment(perName, nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete deployment: %v", perName)

			// podlist, err := frame.GetDeploymentPodList(dpm)
			// notice: can not use GetDeploymentPodList，Deployment deletion is instantaneous
			// all deployment replicas are deleted，gets the time spent on the deletion
			opts := []client.ListOption{
				client.InNamespace(nsName),
				client.MatchingLabels(dpm.Spec.Selector.MatchLabels),
			}

			// time cost for delete deployment
			startT3 := time.Now()
			for {
				podlist, err := frame.GetPodList(opts...)
				Expect(err).NotTo(HaveOccurred(), "failed to list pod")
				GinkgoWriter.Printf("delete deployment time: %v \n", dpm.Status.ReadyReplicas)
				if int32(len(podlist.Items)) == 0 {
					break
				}
			}
			endT3 := time.Since(startT3)
			GinkgoWriter.Printf("time cost for delete deployment of %v replicas = %v \n", scaReplicas, endT3)
		})
	})
})
