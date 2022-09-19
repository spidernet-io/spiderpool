// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package assignip_test

import (
	"context"
	"strings"
	"time"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

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
				// Create Deployment with types.AnnoPodIPPoolValue and The Pods IP is recorded in the IPPool.
				deploy := common.CreateDeployWithPodAnnoation(frame, deployName, namespace, deployOriginialNum, nic, v4PoolNameList, v6PoolNameList)
				common.CheckPodIpReadyByLabel(frame, deploy.Spec.Selector.MatchLabels, v4PoolNameList, v6PoolNameList)

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

				// IPs removed from IPPool.Spec.excludeIPs can be assigned to Pods.
				if frame.Info.IpV4Enabled {
					originalV4Pool := common.GetIppoolByName(frame, v4PoolName)
					// originalV4Pool := v4PoolObject
					Expect(originalV4Pool).NotTo(BeNil())
					// Remove IPs from IPPool.Spec.excludeIPs
					v4PoolObject := common.GetIppoolByName(frame, v4PoolName)
					v4PoolObject.Spec.ExcludeIPs = []string{}
					Expect(common.PatchIppool(frame, v4PoolObject, originalV4Pool)).To(Succeed(), "failed to update v4 ippool: %v ", v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					originalV6Pool := common.GetIppoolByName(frame, v6PoolName)
					Expect(originalV6Pool).NotTo(BeNil())
					// Remove IPs from IPPool.Spec.excludeIPs
					v6PoolObject := common.GetIppoolByName(frame, v6PoolName)
					v6PoolObject.Spec.ExcludeIPs = []string{}
					Expect(common.PatchIppool(frame, v6PoolObject, originalV6Pool)).To(Succeed(), "failed to update v6 ippool: %v ", v6PoolName)
				}

				// After removing an IP from IPPool.Spec.excludeIPs
				// the IP can be assigned to a pod and a record of that pod IP can be checked in the ippool.
				podList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
				Expect(err).NotTo(HaveOccurred())
				newPodList, err := frame.DeletePodListUntilReady(podList, time.Minute)
				Expect(err).NotTo(HaveOccurred())
				Expect(len(newPodList.Items)).Should(Equal(deployScaleupNum))
				ok2, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, newPodList)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok2).To(BeTrue())

				// Delete the deployment
				Expect(frame.DeleteDeployment(deployName, namespace)).To(Succeed())
				GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", namespace, deployName)
			})
	})
})
