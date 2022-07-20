// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package performance_test

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("performance test case", Serial, Label("performance"), func() {
	var perName, nsName string
	var err error
	var dpm *appsv1.Deployment
	var podlist *corev1.PodList

	BeforeEach(func() {
		// Disable api logging
		GinkgoWriter.Printf("Disable api logging to reduce logging \n")
		frame.EnableLog = false

		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// init performance deployment test name
		perName = "per" + tools.RandomName()

		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})
	Context("Time cost of creating, rebuilding and deleting deployment pods in batches", func() {
		var nic, podIppoolAnnoStr string
		var v4PoolNameList, v6PoolNameList []string

		BeforeEach(func() {
			nic = "eth0"

			DeferCleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
				defer cancel()
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Println("Try to delete v4 pool")
					Expect(common.BatchDeletePoolUntilFinish(frame, v4PoolNameList, ctx)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Println("Try to delete v6 pool")
					Expect(common.BatchDeletePoolUntilFinish(frame, v6PoolNameList, ctx)).NotTo(HaveOccurred())
				}
			})
		})

		DescribeTable("Time cost of creating, rebuilding and deleting deployment pods in batches",
			func(entryName, controllerType string, replicasOrIpNum, iPPoolNum int, overtimeCheck time.Duration) {
				var v4Pool, v6Pool []string
				// pod annotation
				podAnno := types.AnnoPodIPPoolValue{
					NIC: &nic,
				}

				if frame.Info.IpV4Enabled {
					v4PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, iPPoolNum, replicasOrIpNum, true)
					Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool")
					// v4 annotation
					v4Pool = v4PoolNameList[:1]
					podAnno.IPv4Pools = v4Pool
				}
				if frame.Info.IpV6Enabled {
					v6PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, iPPoolNum, replicasOrIpNum, false)
					Expect(err).NotTo(HaveOccurred(), "Failed to create v6 pool")
					// v6 annotation
					v6Pool = v6PoolNameList[:1]
					podAnno.IPv6Pools = v6Pool
				}
				b, e := json.Marshal(podAnno)
				Expect(e).NotTo(HaveOccurred())
				podIppoolAnnoStr = string(b)

				switch {
				// Generate deployment object
				case controllerType == common.DeploymentNameString:
					dpm = common.GenerateExampleDeploymentYaml(perName, nsName, int32(replicasOrIpNum))
					// Specify ippool by annotation
					dpm.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
				default:
					Fail("Input variable is not valid")
				}

				// Calculate the time cost of creating a controller until completion
				startT1 := time.Now()

				// Create deployment until ready
				GinkgoWriter.Printf("Try to create controller %v: %v/%v,replicas is %v \n", controllerType, nsName, perName, int32(replicasOrIpNum))
				dpm, err = frame.CreateDeploymentUntilReady(dpm, overtimeCheck)
				Expect(err).NotTo(HaveOccurred(), "Failed to create controller %v : %v/%v, reason=%v", controllerType, nsName, perName, err)
				Expect(dpm).NotTo(BeNil())

				// Get pod list
				podlist, err = frame.GetPodListByLabel(dpm.Spec.Template.Labels)
				Expect(err).NotTo(HaveOccurred(), "Failed to get pod list, reason=%v", err)
				Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))
				GinkgoWriter.Printf("Successfully obtained restarted pod %v/%v the replicas number is %v \n", nsName, perName, len(podlist.Items))

				// Succeeded to assign ipv4、ipv6 ip for pod
				err = frame.CheckPodListIpReady(podlist)
				Expect(err).NotTo(HaveOccurred(), "Failed to check pod list IP ready, reason=%v", err)
				GinkgoWriter.Printf("Succeeded to assign ipv4、ipv6 ip for pod %v/%v \n", nsName, perName)

				// Check pod IP recorded in IPPool
				ok, _, _, e := common.CheckPodIpRecordInIppool(frame, v4Pool, v6Pool, podlist)
				Expect(e).NotTo(HaveOccurred(), "Failed to check pod IP record in IPPool, reason = %v", e)
				Expect(ok).To(BeTrue())
				GinkgoWriter.Printf("Pod %v/%v IP recorded in IPPool %v, %v \n", nsName, perName, v4Pool, v6Pool)
				endT1 := time.Since(startT1)

				// Calculate the rebuild time cost of controller until completion
				startT2 := time.Now()
				err = frame.RestartDeploymentPodUntilReady(perName, nsName, overtimeCheck)
				Expect(err).NotTo(HaveOccurred(), "Failed to rebuild controller %v: %v/%v, maybe GC go wrong , reason=%v ", controllerType, nsName, perName, err)
				GinkgoWriter.Printf("Successfully rebuild controller %v: %v/%v, rebuild replicas as %v \n", controllerType, nsName, perName, replicasOrIpNum)

				// Get the rebuild pod list
				podlist, err = frame.GetPodListByLabel(dpm.Spec.Template.Labels)
				Expect(err).NotTo(HaveOccurred(), "Failed to get pod list ,reason = %v \n", err)
				Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))
				GinkgoWriter.Printf("Successfully obtained restarted pod %v/%v the replicas number is %v", nsName, perName, len(podlist.Items))

				// Succeeded to assign ipv4、ipv6 ip for pod
				err = frame.CheckPodListIpReady(podlist)
				Expect(err).NotTo(HaveOccurred(), "Failed to check ipv4 or ipv6 ,reason=%v \n", err)
				GinkgoWriter.Printf("Succeeded to assign ipv4、ipv6 ip for pod %v/%v \n", nsName, perName)

				// Check pod IP recorded in IPPool
				ok, _, _, e = common.CheckPodIpRecordInIppool(frame, v4Pool, v6Pool, podlist)
				Expect(e).NotTo(HaveOccurred(), "Failed to check pod IP record in IPPool ,reason=%v \n", e)
				Expect(ok).To(BeTrue())
				GinkgoWriter.Printf("Pod %v/%v IP recorded in IPPool %v, %v \n", nsName, perName, v4Pool, v6Pool)
				endT2 := time.Since(startT2)

				// Calculate the time cost of deleting the controller until completion
				startT3 := time.Now()
				Expect(frame.DeleteDeploymentUntilFinish(perName, nsName, overtimeCheck)).To(Succeed())
				GinkgoWriter.Printf("Successfully removed the controller %v:%v/%v \n", controllerType, nsName, perName)

				// Check if the pod IP in IPPool reclaimed normally
				Expect(common.WaitIPReclaimedFinish(frame, v4Pool, v6Pool, podlist, time.Minute)).To(Succeed())
				GinkgoWriter.Println("Pod IP successfully released")
				endT3 := time.Since(startT3)

				// Output the performance results
				GinkgoWriter.Printf("When the cluster has %v ippools, the time cost of creating %v replicas of the %v:%v/%v is %v \n", iPPoolNum, replicasOrIpNum, controllerType, nsName, perName, endT1)
				GinkgoWriter.Printf("When the cluster has %v ippools, the time cost of rebuilding %v replicas of the %v:%v/%v is %v \n", iPPoolNum, replicasOrIpNum, controllerType, nsName, perName, endT2)
				GinkgoWriter.Printf("When the cluster has %v ippools, the time cost of deleting %v replicas of the %v:%v/%v is %v \n", iPPoolNum, replicasOrIpNum, controllerType, nsName, perName, endT3)

				// Attaching Data to Reports
				AddReportEntry(entryName,
					fmt.Sprintf(`{ "controllerType" : "%s", "replicas": %d, "iPPoolNumber": %d , "createTime": %d , "rebuildTime": %d, "deleteTime": %d }`,
						controllerType, replicasOrIpNum, iPPoolNum, int(endT1.Seconds()), int(endT2.Seconds()), int(endT3.Seconds())))
			},

			// When the IP and Pod are 1:1, I want all Pods to run successfully.
			// But I shouldn't change the global default ippool, so I need to generate an ippool.
			Entry("Time cost of creating, rebuilding and deleting deployment pods in batches when the cluster has only one ippools",
				Label("P00002"), "Performance Results", common.DeploymentNameString, 60, 1, time.Minute*4),
			Entry("Time cost of creating, rebuilding and deleting deployment pods in batches when clusters have many ippools",
				Label("P00002"), "Multi-ippool Performance Results", common.DeploymentNameString, 60, 1000, time.Minute*5),
		)
	})
})
