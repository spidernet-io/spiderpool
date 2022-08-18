// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippoolcr_test

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test ippool CR", Label("ippoolCR"), func() {
	var nsName string

	BeforeEach(func() {
		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, time.Second*10)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	Context("test ippool CR", func() {
		var v4PoolName, v4PoolName1, v6PoolName, v6PoolName1, nic, podAnnoStr, deployName string
		var v4PoolObj, v4PoolObj1, v6PoolObj, v6PoolObj1 *spiderpoolv1.SpiderIPPool
		var v4PoolNameList, v6PoolNameList []string
		var disable = new(bool)

		BeforeEach(func() {
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(5)
				Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())
				// create ipv4 pool
				createIPPool(v4PoolObj)
				v4PoolNameList = append(v4PoolNameList, v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(5)
				Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
				// create ipv6 pool
				createIPPool(v6PoolObj)
				v6PoolNameList = append(v6PoolNameList, v6PoolName)
			}
			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					deleteIPPoolUntilFinish(v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					deleteIPPoolUntilFinish(v6PoolName)
				}
			})
		})

		It(" fails to append an ip that already exists in another ippool to the ippool",
			Pending, Label("D00001"), func() {
				// create ippool with the same ip with the former
				if frame.Info.IpV4Enabled {

					GinkgoWriter.Printf("create v4 ippool with same ips %v\n", v4PoolObj.Spec.IPs)
					v4PoolName1, v4PoolObj1 = common.GenerateExampleIpv4poolObject(5)
					v4PoolObj1.Spec.Subnet = v4PoolObj.Spec.Subnet
					v4PoolObj1.Spec.IPs = v4PoolObj.Spec.IPs

					Expect(common.CreateIppool(frame, v4PoolObj1)).NotTo(Succeed())
					GinkgoWriter.Printf("failed to create v4 ippool %v with the same ip with another ippool %v\n", v4PoolName1, v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Printf("create v6 ippool with same ips %v\n", v6PoolObj.Spec.IPs)
					v6PoolName1, v6PoolObj1 = common.GenerateExampleIpv6poolObject(5)
					v6PoolObj1.Spec.Subnet = v6PoolObj.Spec.Subnet
					v6PoolObj1.Spec.IPs = v6PoolObj.Spec.IPs

					Expect(common.CreateIppool(frame, v6PoolObj1)).NotTo(Succeed())
					GinkgoWriter.Printf("failed to create v6 ippool %v with the same ip with another ippool %v\n", v6PoolName1, v6PoolName)
				}
			})
		It(`a "true" value of ippool.Spec.disabled should fobide IP allocation, but still allow ip deallocation`, Label("D00004", "D00005"), Pending, func() {
			// pod annotations
			nic = "eth0"
			deployName = "deploy" + tools.RandomName()
			*disable = true
			podAnno := types.AnnoPodIPPoolValue{
				NIC: &nic,
			}
			if frame.Info.IpV4Enabled {
				podAnno.IPv4Pools = v4PoolNameList
			}
			if frame.Info.IpV6Enabled {
				podAnno.IPv6Pools = v6PoolNameList
			}
			b, e := json.Marshal(podAnno)
			Expect(e).NotTo(HaveOccurred())
			podAnnoStr = string(b)

			// generate deployment yaml
			deployYaml := common.GenerateExampleDeploymentYaml(deployName, nsName, int32(3))
			deployYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podAnnoStr}
			Expect(deployYaml).NotTo(BeNil())

			// create deployment until ready
			deploy, err := frame.CreateDeploymentUntilReady(deployYaml, time.Minute)
			Expect(err).NotTo(HaveOccurred())

			// get pod list
			podList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
			Expect(err).NotTo(HaveOccurred())

			// check pod ip record in ippool
			ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			// delete ippool (D00004)
			if frame.Info.IpV4Enabled {
				Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(Succeed())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(Succeed())
			}

			// check pod ip record in ippool again (D00004)
			GinkgoWriter.Println("check podIP record in ippool again")
			ok2, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
			Expect(err).NotTo(HaveOccurred())
			Expect(ok2).To(BeTrue())

			// set iPv4/iPv6 PoolObj.Spec.Disable to true
			if frame.Info.IpV4Enabled {
				v4PoolObj = common.GetIppoolByName(frame, v4PoolName)
				v4PoolObj.Spec.Disable = disable
				err = common.UpdateIppool(frame, v4PoolObj)
				Expect(err).NotTo(HaveOccurred(), "Failed to update v4PoolObj.Spec.Disable form `false` to `true` for v4 pool")
				GinkgoWriter.Printf("Succeeded to update %v.Spec.Disable form `false` to `true` for v4 pool \n", v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				v6PoolObj = common.GetIppoolByName(frame, v6PoolName)
				v6PoolObj.Spec.Disable = disable
				err := common.UpdateIppool(frame, v6PoolObj)
				Expect(err).NotTo(HaveOccurred(), "Failed to update v6PoolObj.Spec.Disable form `false` to `true` for v6 pool")
				GinkgoWriter.Println("Succeeded to update %v.Spec.Disable form `false` to `true` for v6 pool \n", v6PoolName)
			}

			// wait for the created new pod
			ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
			defer cancel1()
			pods, _, err := common.ScaleDeployUntilExpectedReplicas(frame, deploy, 5, ctx1)
			Expect(err).NotTo(HaveOccurred(), "Failed to scale deployment")

			ctx2, cancel2 := context.WithTimeout(context.Background(), time.Minute)
			defer cancel2()
			for _, pod := range pods {
				Expect(frame.WaitExceptEventOccurred(ctx2, common.PodEventKind, pod.Name, pod.Namespace, common.CNIFailedToSetUpNetwork)).To(Succeed())
				GinkgoWriter.Printf("Pod %v/%v IP allocation failed when iPv4/iPv6 PoolObj.Spec.Disable is true", pod.Namespace, pod.Name)
			}

			// try to delete deployment
			Expect(frame.DeleteDeploymentUntilFinish(deployName, nsName, time.Minute)).To(Succeed())
			GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", nsName, deployName)

			// Check that the pod ip in the ippool is reclaimed properly
			Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podList, time.Minute)).To(Succeed())
			GinkgoWriter.Println("Pod IP is successfully released")
		})
	})

	Context("create and delete batch of ippool", func() {
		const ippoolNumber = 10
		const ipNum = 2
		It("create and delete batch of ippool and check time cost",
			Label("D00006"), func() {
				if frame.Info.IpV4Enabled {
					// batch create ipv4 ippool
					startT1 := time.Now()
					ipv4PoolNameList, err := common.BatchCreateIppoolWithSpecifiedIPNumber(frame, ippoolNumber, ipNum, true)
					Expect(err).NotTo(HaveOccurred())
					endT1 := time.Since(startT1)
					GinkgoWriter.Printf("time cost for create  %v ipv4 ippool %v \n", ippoolNumber, endT1)
					// batch delete ipv4 ippool
					startT2 := time.Now()

					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					errdel := common.BatchDeletePoolUntilFinish(frame, ipv4PoolNameList, ctx)
					Expect(errdel).NotTo(HaveOccurred())
					endT2 := time.Since(startT2)
					GinkgoWriter.Printf("time cost for delete  %v ipv4 ippool %v \n", ippoolNumber, endT2)
				}

				if frame.Info.IpV6Enabled {
					// batch create ipv6 ippool
					startT3 := time.Now()
					ipv6PoolNameList, err := common.BatchCreateIppoolWithSpecifiedIPNumber(frame, ippoolNumber, ipNum, false)
					Expect(err).NotTo(HaveOccurred())
					endT3 := time.Since(startT3)
					GinkgoWriter.Printf("time cost for create  %v ipv6 ippool %v \n", ippoolNumber, endT3)
					// batch delete ipv6 ippool
					startT4 := time.Now()
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					errdel := common.BatchDeletePoolUntilFinish(frame, ipv6PoolNameList, ctx)
					Expect(errdel).NotTo(HaveOccurred())
					endT4 := time.Since(startT4)
					GinkgoWriter.Printf("time cost for delete  %v ipv6 ippool %v \n", ippoolNumber, endT4)
				}
			})
	})
})

func deleteIPPoolUntilFinish(poolName string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	GinkgoWriter.Printf("Delete ippool %v\n", poolName)
	Expect(common.DeleteIPPoolUntilFinish(frame, poolName, ctx)).To(Succeed())
}

func createIPPool(IPPoolObj *spiderpoolv1.SpiderIPPool) {
	GinkgoWriter.Printf("Create ippool %v\n", IPPoolObj.Name)
	Expect(common.CreateIppool(frame, IPPoolObj)).To(Succeed())
	GinkgoWriter.Printf("Succeeded to create ippool %v \n", IPPoolObj.Name)
}
