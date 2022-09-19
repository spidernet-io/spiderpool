// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippoolcr_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test ippool CR", Label("ippoolCR"), func() {
	var nsName string
	var v4PoolName, v6PoolName, nic, deployName string
	var v4PoolObj, v6PoolObj *spiderpoolv1.SpiderIPPool
	var v4PoolNameList, v6PoolNameList []string
	var disable = new(bool)

	BeforeEach(func() {
		// Init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, time.Second*10)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// Create IPv4 pools and IPv6 pools
		if frame.Info.IpV4Enabled {
			v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(5)
			Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())
			Expect(common.CreateIppool(frame, v4PoolObj)).To(Succeed())
			GinkgoWriter.Printf("Succeeded to create ippool %v \n", v4PoolObj.Name)
			v4PoolNameList = append(v4PoolNameList, v4PoolName)
		}
		if frame.Info.IpV6Enabled {
			v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(5)
			Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
			Expect(common.CreateIppool(frame, v6PoolObj)).To(Succeed())
			GinkgoWriter.Printf("Succeeded to create ippool %v \n", v6PoolObj.Name)
			v6PoolNameList = append(v6PoolNameList, v6PoolName)
		}

		// Clean test ENV
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace %v", nsName)

			// Delete IPv4 pools and IPv6 pools
			GinkgoWriter.Printf("Delete IPv4 pools %v and IPv6 pools %v \n", v4PoolName, v6PoolName)
			if frame.Info.IpV4Enabled {
				Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
			}
		})
	})

	Context("test ippool CR", func() {
		var v4PoolName1, v6PoolName1 string
		var v4PoolObj1, v6PoolObj1 *spiderpoolv1.SpiderIPPool

		It("fails to append an ip that already exists in another ippool to the ippool",
			Label("D00001"), func() {
				// In IPv4 and IPv6 scenarios
				// Create an IPPool with the same IPs as the former.
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Printf("Create an ipv4 IPPool with the same IPs %v \n", v4PoolObj.Spec.IPs)
					v4PoolName1, v4PoolObj1 = common.GenerateExampleIpv4poolObject(5)
					v4PoolObj1.Spec.Subnet = v4PoolObj.Spec.Subnet
					v4PoolObj1.Spec.IPs = v4PoolObj.Spec.IPs

					Expect(common.CreateIppool(frame, v4PoolObj1)).NotTo(Succeed())
					GinkgoWriter.Printf("Failed to create an IPv4 IPPool %v with the same IP as another IPPool %v \n", v4PoolName1, v4PoolName)
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Printf("Create an IPv6 IPPool with the same IPs %v \n", v6PoolObj.Spec.IPs)
					v6PoolName1, v6PoolObj1 = common.GenerateExampleIpv6poolObject(5)
					v6PoolObj1.Spec.Subnet = v6PoolObj.Spec.Subnet
					v6PoolObj1.Spec.IPs = v6PoolObj.Spec.IPs

					Expect(common.CreateIppool(frame, v6PoolObj1)).NotTo(Succeed())
					GinkgoWriter.Printf("Failed to create an IPv6 IPPool %v with the same IP as another IPPool %v \n", v6PoolName1, v6PoolName)
				}
			})
	})

	It(`a "true" value of ippool.Spec.disabled should fobide IP allocation, but still allow ip deallocation`, Label("D00004", "D00005"), Pending, func() {
		var (
			deployOriginialNum int = 1
			deployScaleupNum   int = 2
		)
		// ippool.Spec.disabled set to true
		*disable = true

		nic = "eth0"
		deployName = "deploy" + tools.RandomName()

		// Create Deployment with types.AnnoPodIPPoolValue and The Pods IP is recorded in the IPPool.
		deploy := common.CreateDeployWithPodAnnoation(frame, deployName, nsName, deployOriginialNum, nic, v4PoolNameList, v6PoolNameList)
		podList := common.CheckPodIpReadyByLabel(frame, deploy.Spec.Selector.MatchLabels, v4PoolNameList, v6PoolNameList)

		// D00004: Failed to delete an IPPool whose IP is not de-allocated at all
		// Delete IPPool when the IP in IPPool has already been allocated and expect the deletion to fail
		if frame.Info.IpV4Enabled {
			Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(Succeed())
		}
		if frame.Info.IpV6Enabled {
			Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(Succeed())
		}

		// Check the Pod IP recorded in IPPool again
		GinkgoWriter.Println("check podIP record in ippool again")
		ok2, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podList)
		Expect(err).NotTo(HaveOccurred())
		Expect(ok2).To(BeTrue())

		// D00005: A "true" value of IPPool.Spec.disabled should forbid IP allocation, but still allow ip de-allocation
		// Set the value of IPPool.Spec.disabled to "true"
		if frame.Info.IpV4Enabled {
			desiredV4PoolObj := common.GetIppoolByName(frame, v4PoolName)
			desiredV4PoolObj.Spec.Disable = disable
			err = common.PatchIppool(frame, desiredV4PoolObj, v4PoolObj)
			Expect(err).NotTo(HaveOccurred(), "Failed to update %v.Spec.Disable form `false` to `true` for v4 pool", v4PoolName)
			GinkgoWriter.Printf("Succeeded to update %v.Spec.Disable form `false` to `true` for v4 pool \n", v4PoolName)
		}
		if frame.Info.IpV6Enabled {
			desiredV6PoolObj := common.GetIppoolByName(frame, v6PoolName)
			desiredV6PoolObj.Spec.Disable = disable
			err := common.PatchIppool(frame, desiredV6PoolObj, v6PoolObj)
			Expect(err).NotTo(HaveOccurred(), "Failed to update %v.Spec.Disable form `false` to `true` for v6 pool", v6PoolName)
			GinkgoWriter.Println("Succeeded to update %v.Spec.Disable form `false` to `true` for v6 pool \n", v6PoolName)
		}

		// The value of IPPool.Spec.disabled is "true" and the Scale deployment
		ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute)
		defer cancel1()
		pods, _, err := common.ScaleDeployUntilExpectedReplicas(frame, deploy, deployScaleupNum, ctx1)
		Expect(err).NotTo(HaveOccurred(), "Failed to scale deployment")

		// Failed to run pod and Get the Pod Scale failure Event
		ctx2, cancel2 := context.WithTimeout(context.Background(), time.Minute)
		defer cancel2()
		for _, pod := range pods {
			Expect(frame.WaitExceptEventOccurred(ctx2, common.PodEventKind, pod.Name, pod.Namespace, common.CNIFailedToSetUpNetwork)).To(Succeed())
			GinkgoWriter.Printf("Pod %v/%v IP allocation failed when iPv4/iPv6 PoolObj.Spec.Disable is true", pod.Namespace, pod.Name)
		}

		// Delete the deployment and then check that the Pod IP in the IPPool has been reclaimed correctly.
		Expect(frame.DeleteDeploymentUntilFinish(deployName, nsName, time.Minute)).To(Succeed())
		GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", nsName, deployName)
		Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podList, time.Minute)).To(Succeed())
		GinkgoWriter.Printf("The Pod %v/%v IP in the IPPool was reclaimed correctly \n", nsName, deployName)
	})

	It("add a route with `routes` and `gateway` fields in the ippool spec", Label("D00002", "D00003"), func() {
		podName := "pod" + tools.RandomName()
		annoPodIPPool := types.AnnoPodIPPoolValue{}
		var v4Gateway, v6Gateway, v4Dst, v6Dst, v4Via, v6Via string
		var v4InvalidGateway, v6InvalidGateway string
		var v4Pool, v6Pool *spiderpoolv1.SpiderIPPool

		// Generate Invalid Gateway and Dst
		v4InvalidGateway = common.GenerateExampleIpv4Gateway()
		v6InvalidGateway = common.GenerateExampleIpv6Gateway()

		// Generate valid Gateway and Dst
		if frame.Info.IpV4Enabled {
			annoPodIPPool.IPv4Pools = []string{v4PoolName}
			v4Gateway = strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "1"
			v4Dst = strings.Split(v4PoolObj.Spec.Subnet, ".")[0] + "." + strings.Split(v4PoolObj.Spec.Subnet, "/")[1] + ".0.0/16"
			v4Via = strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "254"
		}
		if frame.Info.IpV6Enabled {
			annoPodIPPool.IPv6Pools = []string{v6PoolName}
			v6Gateway = strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "1"
			v6Dst = strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "/32"
			v6Via = strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "fe"
		}

		if frame.Info.IpV4Enabled {
			By("update v4 pool: invalid `gateway` and valid `route`")
			// Get the IPv4 pool, use a invalid "gateway" and valid "route" to update the IPv4 ippool and expect the update to fails.
			originalV4Pool := common.GetIppoolByName(frame, v4PoolName)
			v4Pool = common.GetIppoolByName(frame, v4PoolName)
			Expect(v4Pool).NotTo(BeNil())

			v4Pool.Spec.Gateway = &v4InvalidGateway
			route := spiderpoolv1.Route{
				Dst: v4Dst,
				Gw:  v4Via,
			}
			v4Pool.Spec.Routes = []spiderpoolv1.Route{route}
			Expect(common.PatchIppool(frame, v4Pool, originalV4Pool)).NotTo(Succeed(), "error: we expect failed to update v4 ippool: %v with invalid gateway: %v, and valid route: %+v\n", v4PoolName, v4InvalidGateway, route)

			By("update v4 pool: valid `gateway` and invalid `route`")
			// Get the IPv4 pool, use a valid "gateway" and invalid "route" to update the IPv4 ippool and expect the update to fails.
			v4Pool = common.GetIppoolByName(frame, v4PoolName)
			Expect(v4Pool).NotTo(BeNil())

			v4Pool.Spec.Gateway = &v4Gateway
			route = spiderpoolv1.Route{
				Dst: v4Dst,
				Gw:  v4InvalidGateway,
			}
			v4Pool.Spec.Routes = []spiderpoolv1.Route{route}
			Expect(common.PatchIppool(frame, v4Pool, originalV4Pool)).NotTo(Succeed(), "error: we expect failed to update v4 ippool: %v with valid gateway: %v, and invalid route: %+v\n", v4PoolName, v4InvalidGateway, route)

			By("update v4 pool: valid `gateway` and `route`")
			// Get the IPv4 pool, use a valid "gateway" and "route" to update the IPv4 ippool and expect the update to succeed.
			v4Pool = common.GetIppoolByName(frame, v4PoolName)
			Expect(v4Pool).NotTo(BeNil())

			v4Pool.Spec.Gateway = &v4Gateway
			route = spiderpoolv1.Route{
				Dst: v4Dst,
				Gw:  v4Via,
			}
			v4Pool.Spec.Routes = []spiderpoolv1.Route{route}
			Expect(common.PatchIppool(frame, v4Pool, originalV4Pool)).To(Succeed(), "failed to update v4 ippool: %v with valid gateway: %v, and route: %+v\n", v4PoolName, v4Gateway, route)
		}
		if frame.Info.IpV6Enabled {
			By("update v6 pool: invalid `gateway` and valid `route`")
			// Get the IPv6 pool, use a invalid "gateway" and valid "route" to update the IPv6 ippool and expect the update to fails.
			originalV6Pool := common.GetIppoolByName(frame, v6PoolName)
			v6Pool = common.GetIppoolByName(frame, v6PoolName)
			Expect(v6Pool).NotTo(BeNil())

			v6Pool.Spec.Gateway = &v6InvalidGateway
			route := spiderpoolv1.Route{
				Dst: v6Dst,
				Gw:  v6Via,
			}
			v6Pool.Spec.Routes = []spiderpoolv1.Route{route}
			Expect(common.PatchIppool(frame, v6Pool, originalV6Pool)).NotTo(Succeed(), "error: we expect failed to update v6 ippool: %v with invalid gateway: %v, and valid route: %+v\n", v6PoolName, v6InvalidGateway, route)

			By("update v6 pool: valid `gateway` and invalid `route`")
			// Get the IPv6 pool, use a valid "gateway" and invalid "route" to update the IPv6 ippool and expect the update to fails.
			v6Pool = common.GetIppoolByName(frame, v6PoolName)
			Expect(v6Pool).NotTo(BeNil())

			v6Pool.Spec.Gateway = &v6Gateway
			route = spiderpoolv1.Route{
				Dst: v6Dst,
				Gw:  v6InvalidGateway,
			}
			v6Pool.Spec.Routes = []spiderpoolv1.Route{route}
			Expect(common.PatchIppool(frame, v6Pool, originalV6Pool)).NotTo(Succeed(), "error: we expect failed to update v6 ippool: %v with valid gateway: %v, and invalid route: %+v\n", v6PoolName, v6InvalidGateway, route)

			By("update v6 pool: valid `gateway` and `route`")
			// Get the IPv6 pool, use a valid "gateway" and "route" to update the IPv6 ippool and expect the update to succeed.
			v6Pool = common.GetIppoolByName(frame, v6PoolName)
			Expect(v6Pool).NotTo(BeNil())

			v6Pool.Spec.Gateway = &v6Gateway
			route = spiderpoolv1.Route{
				Dst: v6Dst,
				Gw:  v6Via,
			}
			v6Pool.Spec.Routes = []spiderpoolv1.Route{route}
			Expect(common.PatchIppool(frame, v6Pool, originalV6Pool)).To(Succeed(), "failed to update v6 ippool: %v with valid gateway: %v, and route: %+v\n", v6PoolName, v6Gateway, route)
		}

		// The ippool specified by annotation then creates the pod
		common.CreatePodWithAnnoPodIPPool(frame, podName, nsName, annoPodIPPool)

		// Check whether the route information is effective
		GinkgoWriter.Println("check whether the route information is effective")
		if frame.Info.IpV4Enabled {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			checkGatewayCommand := fmt.Sprintf("ip r | grep 'default via %s'", v4Gateway)
			checkRouteCommand := fmt.Sprintf("ip r | grep '%s via %s'", v4Dst, v4Via)
			_, err := frame.ExecCommandInPod(podName, nsName, checkGatewayCommand, ctx)
			Expect(err).NotTo(HaveOccurred())
			_, err = frame.ExecCommandInPod(podName, nsName, checkRouteCommand, ctx)
			Expect(err).NotTo(HaveOccurred())
		}
		if frame.Info.IpV6Enabled {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			checkGatewayCommand := fmt.Sprintf("ip -6 r | grep 'default via %s'", v6Gateway)
			checkRouteCommand := fmt.Sprintf("ip -6 r | grep '%s via %s'", v6Dst, v6Via)
			_, err := frame.ExecCommandInPod(podName, nsName, checkGatewayCommand, ctx)
			Expect(err).NotTo(HaveOccurred())
			_, err = frame.ExecCommandInPod(podName, nsName, checkRouteCommand, ctx)
			Expect(err).NotTo(HaveOccurred())
		}

		// delete pod
		Expect(frame.DeletePod(podName, nsName)).To(Succeed())
	})

	It("create and delete batch of ippool and check time cost", Label("D00006"), func() {
		const ippoolNumber = 10
		const ipNum = 2

		if frame.Info.IpV4Enabled {
			// Create and delete a batch of IPv4 IPPools and check the time cost
			startT1 := time.Now()
			ipv4PoolNameList, err := common.BatchCreateIppoolWithSpecifiedIPNumber(frame, ippoolNumber, ipNum, true)
			Expect(err).NotTo(HaveOccurred())
			endT1 := time.Since(startT1)
			GinkgoWriter.Printf("Time cost to create %v ipv4 ippools is %v \n", ippoolNumber, endT1)

			startT2 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			errdel := common.BatchDeletePoolUntilFinish(frame, ipv4PoolNameList, ctx)
			Expect(errdel).NotTo(HaveOccurred())
			endT2 := time.Since(startT2)
			GinkgoWriter.Printf("Time cost to delete %v ipv4 ippools is %v \n", ippoolNumber, endT2)
		}

		if frame.Info.IpV6Enabled {
			// Create and delete a batch of IPv6 IPPools and check the time cost
			startT3 := time.Now()
			ipv6PoolNameList, err := common.BatchCreateIppoolWithSpecifiedIPNumber(frame, ippoolNumber, ipNum, false)
			Expect(err).NotTo(HaveOccurred())
			endT3 := time.Since(startT3)
			GinkgoWriter.Printf("Time cost to create %v ipv6 ippools is %v \n", ippoolNumber, endT3)

			startT4 := time.Now()
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			errdel := common.BatchDeletePoolUntilFinish(frame, ipv6PoolNameList, ctx)
			Expect(errdel).NotTo(HaveOccurred())
			endT4 := time.Since(startT4)
			GinkgoWriter.Printf("Time cost to delete %v ipv6 ippools is %v \n", ippoolNumber, endT4)
		}
	})
})
