// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reservedip_test

import (
	"context"

	v1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test reservedIP", Label("reservedIP"), func() {
	var nsName, DeployName, v4PoolName, v6PoolName, v4ReservedIpName, v6ReservedIpName string
	var v4PoolNameList, v6PoolNameList []string
	var iPv4PoolObj, iPv6PoolObj *v1.SpiderIPPool
	var v4ReservedIpObj, v6ReservedIpObj *v1.SpiderReservedIP
	var err error
	var v4SubnetName, v6SubnetName string
	var v4SubnetObject, v6SubnetObject *v1.SpiderSubnet

	BeforeEach(func() {
		if frame.Info.SpiderSubnetEnabled {
			// Subnet Adaptation
			if frame.Info.IpV4Enabled {
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(1)
				Expect(v4SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(1)
				Expect(v6SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
			}
		}

		//Init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("Try to create namespace %v \n", nsName)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace %v", nsName)

		// Init test Deployment/Pod name
		DeployName = "sr-pod" + tools.RandomName()

		if frame.Info.IpV4Enabled {
			v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(1)
			if frame.Info.SpiderSubnetEnabled {
				iPv4PoolObj.Spec.Subnet = v4SubnetObject.Spec.Subnet
				iPv4PoolObj.Spec.IPs = v4SubnetObject.Spec.IPs
			}
			err := common.CreateIppool(frame, iPv4PoolObj)
			Expect(err).NotTo(HaveOccurred(), "Failed to create v4 Pool %v \n", v4PoolName)
			GinkgoWriter.Printf("Successfully created v4 Pool: %v \n", v4PoolName)
			v4PoolNameList = append(v4PoolNameList, v4PoolName)
		}

		if frame.Info.IpV6Enabled {
			v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(1)
			if frame.Info.SpiderSubnetEnabled {
				iPv6PoolObj.Spec.Subnet = v6SubnetObject.Spec.Subnet
				iPv6PoolObj.Spec.IPs = v6SubnetObject.Spec.IPs
			}
			err := common.CreateIppool(frame, iPv6PoolObj)
			Expect(err).NotTo(HaveOccurred(), "Failed to create v6 Pool %v \n", v6PoolName)
			GinkgoWriter.Printf("Successfully created v6 Pool: %v \n", v6PoolName)
			v6PoolNameList = append(v6PoolNameList, v6PoolName)
		}

		// Clean test env
		DeferCleanup(func() {
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace %v", nsName)
			GinkgoWriter.Printf("Successful deletion of namespace %v \n", nsName)

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

	It("S00001: an IP who is set in ReservedIP CRD, should not be assigned to a pod; S00003: Failed to set same IP in excludeIPs when an IP is assigned to a pod",
		Label("S00001", "smoke", "S00003"), func() {
			const deployOriginialNum = int(1)

			if frame.Info.IpV4Enabled {
				v4ReservedIpName, v4ReservedIpObj = common.GenerateExampleV4ReservedIpObject(iPv4PoolObj.Spec.IPs)
				err = common.CreateReservedIP(frame, v4ReservedIpObj)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Successfully created v4 reservedIP: %v \n", v4ReservedIpName)
			}

			if frame.Info.IpV6Enabled {
				v6ReservedIpName, v6ReservedIpObj = common.GenerateExampleV6ReservedIpObject(iPv6PoolObj.Spec.IPs)
				err = common.CreateReservedIP(frame, v6ReservedIpObj)
				Expect(err).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Successfully created v6 reservedIP : %v \n", v6ReservedIpName)
			}
			// Generate IPPool annotation string
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, v4PoolNameList, v6PoolNameList)

			// Generate Deployment yaml and annotation
			deployObject := common.GenerateExampleDeploymentYaml(DeployName, nsName, int32(deployOriginialNum))
			deployObject.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
			GinkgoWriter.Printf("Try to create Deployment: %v/%v with annotation %v = %v \n", nsName, DeployName, constant.AnnoPodIPPool, podIppoolAnnoStr)

			// Try to create a Deployment and wait for replicas to meet expectations（because the Pod is not running）
			ctx1, cancel1 := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel1()
			podlist, err := common.CreateDeployUntilExpectedReplicas(frame, deployObject, ctx1)
			Expect(err).NotTo(HaveOccurred())
			Expect(int32(len(podlist.Items))).Should(Equal(*deployObject.Spec.Replicas))

			// Get the Pod creation failure Event
			ctx, cancel := context.WithTimeout(context.Background(), common.EventOccurTimeout)
			defer cancel()
			for _, pod := range podlist.Items {
				Expect(frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, common.CNIFailedToSetUpNetwork)).To(Succeed())
				GinkgoWriter.Printf("IP assignment for Deployment/Pod: %v/%v fails when an IP is set in the ReservedIP CRD. \n", pod.Namespace, pod.Name)
			}

			// Try deleting the reservedIP and check if the reserved IP can be assigned to a pod.
			ctx2, cancel2 := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel2()
			if frame.Info.IpV4Enabled {
				Expect(common.DeleteResverdIPUntilFinish(ctx2, frame, v4ReservedIpName)).To(Succeed())
				GinkgoWriter.Printf("Delete v4 reservedIP: %v successfully \n", v4ReservedIpName)
			}
			if frame.Info.IpV6Enabled {
				Expect(common.DeleteResverdIPUntilFinish(ctx2, frame, v6ReservedIpName)).To(Succeed())
				GinkgoWriter.Printf("Delete v6 reservedIP: %v successfully \n", v6ReservedIpName)
			}

			// After removing the reservedIP, wait for the Pod to restart until running
			err = frame.RestartDeploymentPodUntilReady(DeployName, nsName, common.PodReStartTimeout)
			Expect(err).NotTo(HaveOccurred())

			// Succeeded to assign IPv4、IPv6 ip for Deployment/Pod and Deployment/Pod IP recorded in IPPool
			common.CheckPodIpReadyByLabel(frame, deployObject.Spec.Selector.MatchLabels, v4PoolNameList, v6PoolNameList)
			GinkgoWriter.Printf("Pod %v/%v IP recorded in IPPool %v %v \n", nsName, DeployName, v4PoolNameList, v6PoolNameList)

			// S00003: Failed to set same IP in excludeIPs when an IP is assigned to a Pod
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Printf("Update the v4 IPPool and set the IP %v used by the Pod in the excludeIPs. \n", iPv4PoolObj.Spec.IPs)
				desiredV4PoolObj := common.GetIppoolByName(frame, v4PoolName)
				desiredV4PoolObj.Spec.ExcludeIPs = desiredV4PoolObj.Spec.IPs
				Expect(common.PatchIppool(frame, desiredV4PoolObj, iPv4PoolObj)).NotTo(Succeed())
				GinkgoWriter.Printf("Failed to update v4 IPPool %v when setting the IP %v used by the Pod in the IPPool's excludeIPs \n", v4PoolName, iPv4PoolObj.Spec.IPs)
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Printf("Update the v6 IPPool and set the IP %v used by the Pod in the excludeIPs. \n", iPv6PoolObj.Spec.IPs)
				desiredV6PoolObj := common.GetIppoolByName(frame, v6PoolName)
				desiredV6PoolObj.Spec.ExcludeIPs = desiredV6PoolObj.Spec.IPs
				Expect(common.PatchIppool(frame, desiredV6PoolObj, iPv6PoolObj)).NotTo(Succeed())
				GinkgoWriter.Printf("Failed to update v6 IPPool %v when setting the IP %v used by the Pod in the IPPool's excludeIPs \n", v6PoolName, iPv6PoolObj.Spec.IPs)
			}

			// Delete the deployment
			GinkgoWriter.Printf("Delete Deployment: %v/%v \n", nsName, DeployName)
			Expect(frame.DeleteDeployment(DeployName, nsName)).To(Succeed())
		})
})
