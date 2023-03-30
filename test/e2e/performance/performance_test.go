// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package performance_test

import (
	"context"
	"fmt"
	"time"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("performance test case", Serial, Label("performance"), func() {
	var (
		perName, nsName, v4PoolName, v6PoolName, podIppoolAnnoStr string
		err                                                       error
		dpm                                                       *appsv1.Deployment
		podlist                                                   *corev1.PodList
		iPv4PoolObj, iPv6PoolObj                                  *spiderpoolv2beta1.SpiderIPPool
		v4PoolNameList, v6PoolNameList                            []string
		v4SubnetName, v6SubnetName                                string
		v4SubnetObject, v6SubnetObject                            *spiderpoolv2beta1.SpiderSubnet
	)
	BeforeEach(func() {

		// Init test ENV
		perName = "per" + tools.RandomName()
		nsName = "ns" + tools.RandomName()
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)
		GinkgoWriter.Printf("Successful creation of namespace %v \n", nsName)

		// Disable api logging
		GinkgoWriter.Printf("disable api logging to reduce logging \n")
		frame.EnableLog = false

		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)

			if frame.Info.IpV4Enabled {
				Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
				if frame.Info.SpiderSubnetEnabled {
					Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
				}
			}
			if frame.Info.IpV6Enabled {
				Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
				if frame.Info.SpiderSubnetEnabled {
					Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
				}
			}
		})
	})

	DescribeTable("time cost for creating, rebuilding, deleting deployment pod in batches",
		func(controllerType string, replicas int32, overtimeCheck time.Duration) {
			if frame.Info.SpiderSubnetEnabled {
				// Subnet Adaptation
				if frame.Info.IpV4Enabled {
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(int(replicas))
					Expect(v4SubnetObject).NotTo(BeNil())
					Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(int(replicas))
					Expect(v6SubnetObject).NotTo(BeNil())
					Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
				}
			}
			// Generate Pod.IPPool annotations string and create IPv4Pool and IPV6Pool
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Printf("try to create IPv4pool: %v \n", v4PoolName)
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(int(replicas))
				if frame.Info.SpiderSubnetEnabled {
					err := common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, int(replicas))
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(common.CreateIppool(frame, iPv4PoolObj)).NotTo(HaveOccurred())
				}

				v4PoolNameList = append(v4PoolNameList, v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Printf("try to create IPv6pool: %v \n", v6PoolName)
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(int(replicas))
				if frame.Info.SpiderSubnetEnabled {
					err := common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, int(replicas))
					Expect(err).NotTo(HaveOccurred())
				} else {
					Expect(common.CreateIppool(frame, iPv6PoolObj)).NotTo(HaveOccurred())
				}

				v6PoolNameList = append(v6PoolNameList, v6PoolName)
			}
			podIppoolAnnoStr = common.GeneratePodIPPoolAnnotations(frame, common.NIC1, v4PoolNameList, v6PoolNameList)

			// Generate Deployment yaml with Pod.IPPool annotation
			switch {
			case controllerType == common.OwnerDeployment:
				dpm = common.GenerateExampleDeploymentYaml(perName, nsName, replicas)
				dpm.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			default:
				Fail("input variable is not valid")
			}

			// Calculate the time cost of creating Deployment/Pods in batches
			startT1 := time.Now()
			GinkgoWriter.Printf("Try to create controller %v: %v/%v,replicas is %v \n", controllerType, nsName, perName, replicas)
			dpm, err = frame.CreateDeploymentUntilReady(dpm, overtimeCheck)
			Expect(err).NotTo(HaveOccurred(), "Failed to create controller %v : %v/%v, reason=%v", controllerType, nsName, perName, err)
			Expect(dpm).NotTo(BeNil())

			// The Pods IP is recorded in the IPPool.
			podlist = common.CheckPodIpReadyByLabel(frame, dpm.Spec.Template.Labels, v4PoolNameList, v6PoolNameList)
			GinkgoWriter.Printf("Pod %v/%v IP recorded in IPPool %v %v \n", nsName, perName, v4PoolNameList, v6PoolNameList)

			// check uuid in ippool
			if frame.Info.IpV4Enabled {
				Expect(common.CheckUniqueUuidInSpiderPool(frame, v4PoolName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.CheckUniqueUuidInSpiderPool(frame, v6PoolName)).NotTo(HaveOccurred())
			}
			endT1 := time.Since(startT1)

			// Calculate the time cost of rebuilding Deployment/Pods in batches
			startT2 := time.Now()
			err = frame.RestartDeploymentPodUntilReady(perName, nsName, overtimeCheck)
			Expect(err).NotTo(HaveOccurred(), "Failed to rebuild controller %v: %v/%v, maybe GC go wrong , reason=%v ", controllerType, nsName, perName, err)
			GinkgoWriter.Printf("Succeeded to rebuild controller %v : %v/%v, rebuild replicas is %v \n", controllerType, nsName, perName, replicas)

			// All Pod IPs are still recorded in the IPPool after the deployment rebuild.
			podlist = common.CheckPodIpReadyByLabel(frame, dpm.Spec.Template.Labels, v4PoolNameList, v6PoolNameList)
			GinkgoWriter.Printf("Pod %v/%v IP recorded in IPPool %v %v \n", nsName, perName, v4PoolNameList, v6PoolNameList)

			// check uuid in ippool
			if frame.Info.IpV4Enabled {
				Expect(common.CheckUniqueUuidInSpiderPool(frame, v4PoolName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.CheckUniqueUuidInSpiderPool(frame, v6PoolName)).NotTo(HaveOccurred())
			}
			endT2 := time.Since(startT2)

			// Calculate the time cost of deleting a deployment until the Pod IP is fully reclaimed.
			startT3 := time.Now()
			Expect(frame.DeleteDeploymentUntilFinish(perName, nsName, overtimeCheck)).To(Succeed())
			GinkgoWriter.Printf("delete controller %v:%v/%v success \n", controllerType, nsName, perName)
			Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podlist, time.Minute)).To(Succeed())
			GinkgoWriter.Printf("The Pod %v/%v IP in the IPPool %v,%v was reclaimed correctly \n", nsName, perName, v4PoolNameList, v6PoolNameList)
			endT3 := time.Since(startT3)

			// Output the performance results
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
			Label("P00002"), common.OwnerDeployment, int32(40), time.Minute*4),
	)
})
