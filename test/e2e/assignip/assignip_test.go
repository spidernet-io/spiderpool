// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package assignip_test

import (
	"context"
	"strings"
	"time"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

	"github.com/spidernet-io/spiderpool/pkg/types"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test pod", Label("assignip"), func() {

	Context("fail to run a pod when IP resource of an ippool is exhausted or its IP been set excludeIPs", func() {
		var deployName, v4PoolName, v6PoolName, nic, namespace string
		var v4PoolNameList, v6PoolNameList []string
		var v4PoolObj, v6PoolObj *spiderpoolv1.SpiderIPPool
		var (
			deployOriginialNum int = 1
			deployScaleupNum   int = 2
			ippoolIpNum        int = 2
		)

		BeforeEach(func() {
			// Init test information and create namespace
			nic = "eth0"
			deployName = "deploy" + tools.RandomName()
			namespace = "ns" + tools.RandomName()
			GinkgoWriter.Printf("create namespace %v \n", namespace)
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, time.Second*10)
			Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)

			// Create IPv4Pool and IPV6Pool
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(ippoolIpNum)
				// Add an IP from the IPPool.Spec.IPs to the Spec.excludeIPs.
				v4PoolObj.Spec.ExcludeIPs = strings.Split(v4PoolObj.Spec.IPs[0], "-")[:1]
				v4PoolNameList = append(v4PoolNameList, v4PoolName)
				Expect(common.CreateIppool(frame, v4PoolObj)).To(Succeed())
				GinkgoWriter.Printf("Succeeded to create ippool %v \n", v4PoolObj.Name)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(ippoolIpNum)
				// Add an IP from the IPPool.Spec.IPs to the Spec.excludeIPs.
				v6PoolObj.Spec.ExcludeIPs = strings.Split(v6PoolObj.Spec.IPs[0], "-")[:1]
				v6PoolNameList = append(v6PoolNameList, v6PoolName)
				Expect(common.CreateIppool(frame, v6PoolObj)).To(Succeed())
				GinkgoWriter.Printf("Succeeded to create ippool %v \n", v6PoolObj.Name)
			}

			DeferCleanup(func() {
				GinkgoWriter.Printf("Try to delete namespace %v \n", namespace)
				err := frame.DeleteNamespace(namespace)
				Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)

				GinkgoWriter.Printf("Try to delete IPPool %v, %v \n", v4PoolName, v6PoolName)
				if frame.Info.IpV4Enabled {
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
				}
			})
		})

		It(" fail to run a pod when IP resource of an ippool is exhausted and an IP who is set in excludeIPs field of ippool, should not be assigned to a pod",
			Label("E00008", "S00002"), func() {
				// Generate Pod annotations
				podAnno := types.AnnoPodIPPoolValue{
					NIC: &nic,
				}
				if frame.Info.IpV4Enabled {
					podAnno.IPv4Pools = []string{v4PoolName}
				}
				if frame.Info.IpV6Enabled {
					podAnno.IPv6Pools = []string{v6PoolName}
				}

				// Create Deployment with types.AnnoPodIPPoolValue and The Pods IP is recorded in the IPPool.
				deploy := common.CreateDeployWithPodAnnoation(frame, deployName, namespace, deployOriginialNum, podAnno)
				podList := common.CheckPodIpReadyByLabel(frame, deploy.Spec.Selector.MatchLabels, v4PoolNameList, v6PoolNameList)

				// Scale Deployment to exhaust IP resource
				GinkgoWriter.Println("scale Deployment to exhaust IP resource")
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				addPods, _, e4 := common.ScaleDeployUntilExpectedReplicas(frame, deploy, deployScaleupNum, ctx)
				Expect(e4).NotTo(HaveOccurred())

				// Get the Pod Scale failure Event
				ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
				defer cancel1()
				for _, pod := range addPods {
					Expect(frame.WaitExceptEventOccurred(ctx1, common.PodEventKind, pod.Name, pod.Namespace, common.GetIpamAllocationFailed)).To(Succeed())
					GinkgoWriter.Printf("succeeded to detect the message expected: %v\n", common.GetIpamAllocationFailed)
				}

				// Delete the deployment and then check that the Pod IP in the IPPool has been reclaimed correctly.
				Expect(frame.DeleteDeploymentUntilFinish(deployName, namespace, time.Minute)).To(Succeed())
				GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", namespace, deployName)
				Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podList, time.Minute)).To(Succeed())
				GinkgoWriter.Printf("The Pod %v/%v IP in the IPPool was reclaimed correctly \n", namespace, deployName)
			})
	})
})
