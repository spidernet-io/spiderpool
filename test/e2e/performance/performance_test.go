// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package performance_test

import (
	"encoding/json"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
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
	Context("time cost for creating, rebuilding, deleting deployment pod in batches", func() {
		var v4PoolName, v6PoolName, nic, podIppoolAnnoStr string
		var iPv4PoolObj, iPv6PoolObj *spiderpoolv1.IPPool
		var v4PoolNameList, v6PoolNameList []string
		BeforeEach(func() {
			nic = "eth0"
			DeferCleanup(func() {
				if frame.Info.IpV4Enabled {
					err := common.DeleteIPPoolByName(frame, v4PoolName)
					Expect(err).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					err := common.DeleteIPPoolByName(frame, v6PoolName)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
		DescribeTable("time cost for creating, rebuilding, deleting deployment pod in batches",
			func(controllerType string, replicas int32, overtimeCheck time.Duration) {
				// pod annotation
				podAnno := types.AnnoPodIPPoolValue{
					NIC: &nic,
				}
				if frame.Info.IpV4Enabled {
					v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(int(replicas))
					v4PoolNameList = append(v4PoolNameList, v4PoolName)
					GinkgoWriter.Printf("try to create ipv4pool: %v/%v \n", v4PoolName, iPv4PoolObj)
					err := common.CreateIppool(frame, iPv4PoolObj)
					Expect(err).NotTo(HaveOccurred(), "failed to create ipv4pool: %v \n", v4PoolName)
					// v4 annotation
					podAnno.IPv4Pools = v4PoolNameList
				}
				if frame.Info.IpV6Enabled {
					v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(int(replicas))
					v6PoolNameList = append(v6PoolNameList, v6PoolName)
					GinkgoWriter.Printf("try to create ipv6pool: %v/%v \n", v6PoolName, iPv6PoolObj)
					err := common.CreateIppool(frame, iPv6PoolObj)
					Expect(err).NotTo(HaveOccurred(), "failed to create ipv6pool: %v \n", v6PoolName)
					// v6 annotation
					podAnno.IPv6Pools = v6PoolNameList
				}
				b, e1 := json.Marshal(podAnno)
				Expect(e1).NotTo(HaveOccurred())
				podIppoolAnnoStr = string(b)

				switch {
				// Generate deployment object
				case controllerType == common.DeploymentNameString:
					dpm = common.GenerateExampleDeploymentYaml(perName, nsName, replicas)
					// Specify ippool by annotation
					dpm.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
				default:
					Fail("input variable is not valid")
				}

				// Disable api logging
				GinkgoWriter.Printf("disable api logging to reduce logging \n")
				frame.EnableLog = false

				// Calculate the time cost of creating a controller until completion
				startT1 := time.Now()

				// create deployment until ready
				GinkgoWriter.Printf("try to create controller %v: %v/%v,replicas is %v \n", controllerType, nsName, perName, replicas)
				dpm, err = frame.CreateDeploymentUntilReady(dpm, overtimeCheck)
				Expect(err).NotTo(HaveOccurred(), "failed to create controller %v : %v/%v, reason=%v", controllerType, nsName, perName, err)
				Expect(dpm).NotTo(BeNil())

				// get pod list
				podlist, err = frame.GetPodListByLabel(dpm.Spec.Template.Labels)
				Expect(err).NotTo(HaveOccurred(), "failed to get pod list, reason=%v", err)
				Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))
				GinkgoWriter.Printf("succeeded to get the pod list %v/%v, Replicas is %v \n", nsName, perName, len(podlist.Items))

				// succeeded to assign ipv4、ipv6 ip for pod
				err = frame.CheckPodListIpReady(podlist)
				Expect(err).NotTo(HaveOccurred(), "failed to CheckPodListIpReady, reason=%v", err)
				GinkgoWriter.Printf("succeeded to assign ipv4、ipv6 ip for pod %v/%v \n", nsName, perName)

				// check pod ip recorded in ippool
				ok, _, _, e := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podlist)
				Expect(e).NotTo(HaveOccurred(), "failed to CheckPodIpRecordInIppool, reason=%v", e)
				Expect(ok).To(BeTrue())
				GinkgoWriter.Printf("pod %v/%v ip recorded in ippool %v %v \n", nsName, perName, v4PoolNameList, v6PoolNameList)
				endT1 := time.Since(startT1)

				// Calculate the rebuild time cost of controller until completion
				startT2 := time.Now()
				err = frame.RestartDeploymentPodUntilReady(perName, nsName, overtimeCheck)
				Expect(err).NotTo(HaveOccurred(), "failed to rebuild controller %v: %v/%v, maybe GC go wrong , reason=%v ", controllerType, nsName, perName, err)
				GinkgoWriter.Printf("succeeded to rebuild controller %v : %v/%v, rebuild replicas is %v \n", controllerType, nsName, perName, replicas)

				// Get the rebuild pod list
				podlist, err = frame.GetPodListByLabel(dpm.Spec.Template.Labels)
				Expect(err).NotTo(HaveOccurred(), "failed to get pod list ,reason=%v \n", err)
				Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))
				GinkgoWriter.Printf("succeeded to get the restarted pod %v/%v, replicas is %v \n", nsName, perName, len(podlist.Items))

				// succeeded to assign ipv4、ipv6 ip for pod
				err = frame.CheckPodListIpReady(podlist)
				Expect(err).NotTo(HaveOccurred(), "failed to check ipv4 or ipv6 ,reason=%v \n", err)
				GinkgoWriter.Printf("succeeded to assign ipv4、ipv6 ip for pod %v/%v \n", nsName, perName)

				// check pod ip recorded in ippool
				ok, _, _, e = common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podlist)
				Expect(e).NotTo(HaveOccurred(), "failed to CheckPodIpRecordInIppool ,reason=%v \n", err)
				Expect(ok).To(BeTrue())
				GinkgoWriter.Printf("pod %v/%v ip recorded in ippool %v %v \n", nsName, perName, v4PoolNameList, v6PoolNameList)
				endT2 := time.Since(startT2)

				// Calculate the time cost of deleting the controller until completion
				startT3 := time.Now()
				Expect(frame.DeleteDeploymentUntilFinish(perName, nsName, overtimeCheck)).To(Succeed())
				GinkgoWriter.Printf("delete controller %v:%v/%v success \n", controllerType, nsName, perName)

				// check if the pod ip in ippool reclaimed normally
				GinkgoWriter.Println("check ip is release successfully")
				Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podlist, time.Minute)).To(Succeed())
				endT3 := time.Since(startT3)

				// output the performance results
				GinkgoWriter.Printf("time cost for create  %v: %v/%v of %v replicas = %v \n", controllerType, nsName, perName, replicas, endT1)
				GinkgoWriter.Printf("time cost for rebuild %v: %v/%v of %v replicas = %v \n", controllerType, nsName, perName, replicas, endT2)
				GinkgoWriter.Printf("time cost for delete  %v: %v/%v of %v replicas = %v \n", controllerType, nsName, perName, replicas, endT3)
				// attaching Data to Reports
				AddReportEntry("Performance Results",
					fmt.Sprintf(`{ "controllerType" : "%s", "replicas": %d, "createTime": %d , "rebuildTime": %d, "deleteTime": %d }`,
						controllerType, replicas, int(endT1.Seconds()), int(endT2.Seconds()), int(endT3.Seconds())))
			},
			// TODO (tao.yang), N controller replicas in Ippool for N IP, Through this template complete gc performance closed-loop test together
			Entry("time cost for creating, rebuilding, deleting deployment pod in batches",
				Label("P00002"), common.DeploymentNameString, int32(60), time.Minute*4),
		)
	})
})
