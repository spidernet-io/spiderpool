// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippoolcr_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("test ippool CR", Label("ippoolCR"), func() {
	var nsName string
	var v4PoolName, v6PoolName, deployName string
	var v4PoolObj, v6PoolObj *spiderpoolv2beta1.SpiderIPPool
	var v4PoolNameList, v6PoolNameList []string
	var disable = new(bool)
	var v4SubnetName, v6SubnetName string
	var v4SubnetObject, v6SubnetObject *spiderpoolv2beta1.SpiderSubnet

	BeforeEach(func() {
		if frame.Info.SpiderSubnetEnabled {
			if frame.Info.IpV4Enabled {
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 20)
				Expect(v4SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 20)
				Expect(v6SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
			}
		}
		// Init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// Create IPv4 pools and IPv6 pools
		if frame.Info.IpV4Enabled {
			v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(5)
			Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())
			GinkgoWriter.Printf("Succeeded to create ippool %v \n", v4PoolObj.Name)
			if frame.Info.SpiderSubnetEnabled {
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				err := common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4PoolObj, 5)
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(common.CreateIppool(frame, v4PoolObj)).To(Succeed())
			}
			v4PoolNameList = append(v4PoolNameList, v4PoolName)
		}
		if frame.Info.IpV6Enabled {
			v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(5)
			Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
			GinkgoWriter.Printf("Succeeded to create ippool %v \n", v6PoolObj.Name)
			if frame.Info.SpiderSubnetEnabled {
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				err := common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, v6PoolObj, 5)
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(common.CreateIppool(frame, v6PoolObj)).To(Succeed())
			}

			v6PoolNameList = append(v6PoolNameList, v6PoolName)
		}

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			GinkgoWriter.Println("Clean test ENV")
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace %v, err: %v", nsName, err)

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

	Context("test ippool CR", func() {
		var v4PoolName1, v6PoolName1 string
		var v4PoolObj1, v6PoolObj1 *spiderpoolv2beta1.SpiderIPPool

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
		deployName = "deploy" + tools.RandomName()

		// Create Deployment with types.AnnoPodIPPoolValue and The Pods IP is recorded in the IPPool.
		deploy := common.CreateDeployWithPodAnnoation(frame, deployName, nsName, deployOriginialNum, common.NIC1, v4PoolNameList, v6PoolNameList)
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
			desiredV4PoolObj, err := common.GetIppoolByName(frame, v4PoolName)
			Expect(err).NotTo(HaveOccurred())
			desiredV4PoolObj.Spec.Disable = disable
			err = common.PatchIppool(frame, desiredV4PoolObj, v4PoolObj)
			Expect(err).NotTo(HaveOccurred(), "Failed to update %v.Spec.Disable form `false` to `true` for v4 pool", v4PoolName)
			GinkgoWriter.Printf("Succeeded to update %v.Spec.Disable form `false` to `true` for v4 pool \n", v4PoolName)
		}
		if frame.Info.IpV6Enabled {
			desiredV6PoolObj, err := common.GetIppoolByName(frame, v6PoolName)
			Expect(err).NotTo(HaveOccurred())
			desiredV6PoolObj.Spec.Disable = disable
			err = common.PatchIppool(frame, desiredV6PoolObj, v6PoolObj)
			Expect(err).NotTo(HaveOccurred(), "Failed to update %v.Spec.Disable form `false` to `true` for v6 pool", v6PoolName)
			GinkgoWriter.Println("Succeeded to update %v.Spec.Disable form `false` to `true` for v6 pool \n", v6PoolName)
		}

		// The value of IPPool.Spec.disabled is "true" and the Scale deployment
		ctx1, cancel1 := context.WithTimeout(context.Background(), common.PodReStartTimeout)
		defer cancel1()
		pods, _, err := common.ScaleDeployUntilExpectedReplicas(ctx1, frame, deploy, deployScaleupNum, false)
		Expect(err).NotTo(HaveOccurred(), "Failed to scale deployment")

		// Failed to run pod and Get the Pod Scale failure Event
		ctx2, cancel2 := context.WithTimeout(context.Background(), common.EventOccurTimeout)
		defer cancel2()
		for _, pod := range pods {
			Expect(frame.WaitExceptEventOccurred(ctx2, common.OwnerPod, pod.Name, pod.Namespace, common.CNIFailedToSetUpNetwork)).To(Succeed())
			GinkgoWriter.Printf("Pod %v/%v IP allocation failed when iPv4/iPv6 PoolObj.Spec.Disable is true", pod.Namespace, pod.Name)
		}

		// Delete the deployment and then check that the Pod IP in the IPPool has been reclaimed correctly.
		Expect(frame.DeleteDeploymentUntilFinish(deployName, nsName, common.ResourceDeleteTimeout)).To(Succeed())
		GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", nsName, deployName)
		Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podList, common.IPReclaimTimeout)).To(Succeed())
		GinkgoWriter.Printf("The Pod %v/%v IP in the IPPool was reclaimed correctly \n", nsName, deployName)
	})

	It("add a route with `routes` and `gateway` fields in the ippool spec", Label("D00002", "D00003", "smoke", "A00011"), func() {
		podName1 := "pod" + tools.RandomName()
		podName2 := "pod" + tools.RandomName()
		var v4Gateway, v6Gateway, v4Dst, v6Dst, v4Via, v6Via string
		var v4InvalidGateway, v6InvalidGateway string
		var v4Pool, v6Pool *spiderpoolv2beta1.SpiderIPPool

		// Generate Invalid Gateway and Dst
		v4InvalidGateway = common.GenerateRandomIPV4()
		v6InvalidGateway = common.GenerateRandomIPV6()

		annoPodIPPools := types.AnnoPodIPPoolsValue{
			types.AnnoIPPoolItem{
				NIC: common.NIC1,
			},
		}
		// Generate valid Gateway and Dst
		if frame.Info.IpV4Enabled {
			annoPodIPPools[0].IPv4Pools = []string{v4PoolName}
			v4Gateway = strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "1"
			v4Dst = strings.Split(v4PoolObj.Spec.Subnet, ".")[0] + "." + strings.Split(v4PoolObj.Spec.Subnet, "/")[1] + ".0.0/16"
			v4Via = strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "254"
		}
		if frame.Info.IpV6Enabled {
			annoPodIPPools[0].IPv6Pools = []string{v6PoolName}
			v6Gateway = strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "1"
			v6Dst = strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "/32"
			v6Via = strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "fe"
		}

		if frame.Info.IpV4Enabled {
			By("update v4 pool: invalid `gateway` and valid `route`")
			// Get the IPv4 pool, use a invalid "gateway" and valid "route" to update the IPv4 ippool and expect the update to fails.
			originalV4Pool, err := common.GetIppoolByName(frame, v4PoolName)
			Expect(err).NotTo(HaveOccurred())
			v4Pool, err = common.GetIppoolByName(frame, v4PoolName)
			Expect(err).NotTo(HaveOccurred())

			v4Pool.Spec.Gateway = &v4InvalidGateway
			route := spiderpoolv2beta1.Route{
				Dst: v4Dst,
				Gw:  v4Via,
			}
			v4Pool.Spec.Routes = []spiderpoolv2beta1.Route{route}
			Expect(common.PatchIppool(frame, v4Pool, originalV4Pool)).NotTo(Succeed(), "error: we expect failed to update v4 ippool: %v with invalid gateway: %v, and valid route: %+v\n", v4PoolName, v4InvalidGateway, route)

			By("update v4 pool: valid `gateway` and invalid `route`")
			// Get the IPv4 pool, use a valid "gateway" and invalid "route" to update the IPv4 ippool and expect the update to fails.
			v4Pool, err = common.GetIppoolByName(frame, v4PoolName)
			Expect(err).NotTo(HaveOccurred())

			v4Pool.Spec.Gateway = &v4Gateway
			route = spiderpoolv2beta1.Route{
				Dst: v4Dst,
				Gw:  v4InvalidGateway,
			}
			v4Pool.Spec.Routes = []spiderpoolv2beta1.Route{route}
			Expect(common.PatchIppool(frame, v4Pool, originalV4Pool)).NotTo(Succeed(), "error: we expect failed to update v4 ippool: %v with valid gateway: %v, and invalid route: %+v\n", v4PoolName, v4InvalidGateway, route)

			By("update v4 pool: valid `gateway` and `route`")
			// Get the IPv4 pool, use a valid "gateway" and "route" to update the IPv4 ippool and expect the update to succeed.
			v4Pool, err = common.GetIppoolByName(frame, v4PoolName)
			Expect(err).NotTo(HaveOccurred())

			v4Pool.Spec.Gateway = &v4Gateway
			route = spiderpoolv2beta1.Route{
				Dst: v4Dst,
				Gw:  v4Via,
			}
			v4Pool.Spec.Routes = []spiderpoolv2beta1.Route{route}
			Expect(common.PatchIppool(frame, v4Pool, originalV4Pool)).To(Succeed(), "failed to update v4 ippool: %v with valid gateway: %v, and route: %+v\n", v4PoolName, v4Gateway, route)
		}
		if frame.Info.IpV6Enabled {
			By("update v6 pool: invalid `gateway` and valid `route`")
			// Get the IPv6 pool, use a invalid "gateway" and valid "route" to update the IPv6 ippool and expect the update to fails.
			originalV6Pool, err := common.GetIppoolByName(frame, v6PoolName)
			Expect(err).NotTo(HaveOccurred())
			v6Pool, err = common.GetIppoolByName(frame, v6PoolName)
			Expect(err).NotTo(HaveOccurred())

			v6Pool.Spec.Gateway = &v6InvalidGateway
			route := spiderpoolv2beta1.Route{
				Dst: v6Dst,
				Gw:  v6Via,
			}
			v6Pool.Spec.Routes = []spiderpoolv2beta1.Route{route}
			Expect(common.PatchIppool(frame, v6Pool, originalV6Pool)).NotTo(Succeed(), "error: we expect failed to update v6 ippool: %v with invalid gateway: %v, and valid route: %+v\n", v6PoolName, v6InvalidGateway, route)

			By("update v6 pool: valid `gateway` and invalid `route`")
			// Get the IPv6 pool, use a valid "gateway" and invalid "route" to update the IPv6 ippool and expect the update to fails.
			v6Pool, err = common.GetIppoolByName(frame, v6PoolName)
			Expect(err).NotTo(HaveOccurred())

			v6Pool.Spec.Gateway = &v6Gateway
			route = spiderpoolv2beta1.Route{
				Dst: v6Dst,
				Gw:  v6InvalidGateway,
			}
			v6Pool.Spec.Routes = []spiderpoolv2beta1.Route{route}
			Expect(common.PatchIppool(frame, v6Pool, originalV6Pool)).NotTo(Succeed(), "error: we expect failed to update v6 ippool: %v with valid gateway: %v, and invalid route: %+v\n", v6PoolName, v6InvalidGateway, route)

			By("update v6 pool: valid `gateway` and `route`")
			// Get the IPv6 pool, use a valid "gateway" and "route" to update the IPv6 ippool and expect the update to succeed.
			v6Pool, err = common.GetIppoolByName(frame, v6PoolName)
			Expect(err).NotTo(HaveOccurred())

			v6Pool.Spec.Gateway = &v6Gateway
			route = spiderpoolv2beta1.Route{
				Dst: v6Dst,
				Gw:  v6Via,
			}
			v6Pool.Spec.Routes = []spiderpoolv2beta1.Route{route}
			Expect(common.PatchIppool(frame, v6Pool, originalV6Pool)).To(Succeed(), "failed to update v6 ippool: %v with valid gateway: %v, and route: %+v\n", v6PoolName, v6Gateway, route)
		}

		// The ippool specified by annotation then creates the pod
		// A00011: Use the ippool route with cleanGateway=false in the pod annotation as a default route
		podNameCleanGatewayMap := map[string]bool{
			podName1: false,
			podName2: true,
		}
		for k, v := range podNameCleanGatewayMap {
			annoPodIPPools[0].CleanGateway = v
			b, err := json.Marshal(annoPodIPPools)
			Expect(err).NotTo(HaveOccurred())
			annoPodIPPoolsStr := string(b)
			podYaml := common.GenerateExamplePodYaml(k, nsName)
			podYaml.Annotations = map[string]string{constant.AnnoPodIPPools: annoPodIPPoolsStr}
			common.CreatePodUntilReady(frame, podYaml, k, nsName, common.PodStartTimeout)
		}

		// Check whether the route information is effective
		GinkgoWriter.Println("check whether the route information is effective.")
		if frame.Info.IpV4Enabled {
			for k := range podNameCleanGatewayMap {
				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				checkGatewayCommand := fmt.Sprintf("ip r | grep 'default via %s'", v4Gateway)
				checkRouteCommand := fmt.Sprintf("ip r | grep '%s via %s'", v4Dst, v4Via)
				output1, err1 := frame.ExecCommandInPod(k, nsName, checkGatewayCommand, ctx)
				output2, err2 := frame.ExecCommandInPod(k, nsName, checkRouteCommand, ctx)
				if k == podName1 {
					Expect(output1).NotTo(BeEmpty())
					Expect(err1).NotTo(HaveOccurred())
				}
				if k == podName2 {
					Expect(output1).To(BeEmpty())
					Expect(err1).To(HaveOccurred(), "cleanGateway=true, will not be used as the default route:%v \n", err1)
				}
				Expect(err2).NotTo(HaveOccurred())
				Expect(output2).NotTo(BeEmpty())
			}
		}
		if frame.Info.IpV6Enabled {
			for k := range podNameCleanGatewayMap {
				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				checkGatewayCommand := "ip -6 r show table main | grep -v veth0 | grep 'default via' | awk '{print $3}'"
				checkRouteDstCommand := fmt.Sprintf("ip -6 r show table main | grep -v veth0 | grep 'via' | grep -w '%s' | grep -v 'default' | awk '{print $1}'", common.NIC1)
				checkRouteViaCommand := fmt.Sprintf("ip -6 r show table main | grep -v veth0 | grep 'via' | grep -w '%s' | grep -v 'default' | awk '{print $3}'", common.NIC1)
				effectiveIpv6Gw, err1 := frame.ExecCommandInPod(k, nsName, checkGatewayCommand, ctx)
				effectiveIpv6Dst, err2 := frame.ExecCommandInPod(k, nsName, checkRouteDstCommand, ctx)
				effectiveIpv6Via, err3 := frame.ExecCommandInPod(k, nsName, checkRouteViaCommand, ctx)
				if k == podName1 {
					Expect(common.ContrastIpv6ToIntValues(strings.TrimSpace(string(effectiveIpv6Gw)), v6Gateway)).NotTo(HaveOccurred())
					Expect(err1).NotTo(HaveOccurred(), "failed execute command %v,error:%v", checkGatewayCommand, err1)
				}
				if k == podName2 {
					Expect(effectiveIpv6Gw).To(BeEmpty())
				}
				Expect(common.ContrastIpv6ToIntValues(strings.TrimSpace(string(effectiveIpv6Dst)), v6Dst)).NotTo(HaveOccurred())
				Expect(common.ContrastIpv6ToIntValues(strings.TrimSpace(string(effectiveIpv6Via)), v6Via)).NotTo(HaveOccurred())
				Expect(err2).NotTo(HaveOccurred(), "failed execute command %v,error:%v", checkRouteDstCommand, err2)
				Expect(err3).NotTo(HaveOccurred(), "failed execute command %v,error:%v", checkRouteViaCommand, err3)
			}
		}

		// delete pod
		Expect(frame.DeletePod(podName1, nsName)).To(Succeed())
		Expect(frame.DeletePod(podName2, nsName)).To(Succeed())
	})

	It("create and delete batch of ippool and check time cost", Label("D00006"), func() {
		var ipv4PoolNameList, ipv6PoolNameList []string
		var err error
		const ippoolNumber = 10
		const ipNum = 1

		if frame.Info.IpV4Enabled {
			// Create and delete a batch of IPv4 IPPools and check the time cost
			startT1 := time.Now()
			ipv4PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, ippoolNumber, ipNum, true)
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
			ipv6PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, ippoolNumber, ipNum, false)
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

	Context("Test multusName affinity", func() {
		var namespace, v4PoolName, v6PoolName, dsName, spiderMultusNadName string
		var iPv4PoolObj, iPv6PoolObj *spiderpoolv2beta1.SpiderIPPool
		var v4SubnetName, v6SubnetName string
		var v4SubnetObject, v6SubnetObject *spiderpoolv2beta1.SpiderSubnet
		var spiderMultusConfig *spiderpoolv2beta1.SpiderMultusConfig

		BeforeEach(func() {
			dsName = "ds-" + common.GenerateString(10, true)
			namespace = "ns" + tools.RandomName()
			spiderMultusNadName = "test-multus-" + common.GenerateString(10, true)

			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			GinkgoWriter.Printf("create namespace %v. \n", namespace)
			Expect(err).NotTo(HaveOccurred())

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(1)
				// Associate ip pool with multus name
				iPv4PoolObj.Spec.MultusName = []string{spiderMultusNadName}
				if frame.Info.SpiderSubnetEnabled {
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, len(frame.Info.KindNodeList))
					Expect(v4SubnetObject).NotTo(BeNil())
					Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, len(frame.Info.KindNodeList))
				} else {
					err = common.CreateIppool(frame, iPv4PoolObj)
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to create v4 Pool %v \n", v4PoolName)
			}

			if frame.Info.IpV6Enabled {
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(len(frame.Info.KindNodeList))
				// Associate ip pool with multus name
				iPv6PoolObj.Spec.MultusName = []string{spiderMultusNadName}
				if frame.Info.SpiderSubnetEnabled {
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, len(frame.Info.KindNodeList))
					Expect(v6SubnetObject).NotTo(BeNil())
					Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, len(frame.Info.KindNodeList))
				} else {
					err = common.CreateIppool(frame, iPv6PoolObj)
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to create v6 Pool %v \n", v6PoolName)
			}

			spiderMultusConfig = &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderMultusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: "macvlan",
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
						SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
							IPv4IPPool: []string{v4PoolName},
							IPv6IPPool: []string{v6PoolName},
						},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{},
				},
			}
			GinkgoWriter.Printf("Generate spiderMultusConfig %v \n", spiderMultusConfig)
			Expect(frame.CreateSpiderMultusInstance(spiderMultusConfig)).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Create spidermultus config %v/%v \n", namespace, spiderMultusNadName)

			DeferCleanup(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
					return
				}
				GinkgoWriter.Printf("delete namespace %v. \n", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())

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

		It("IPPool can be allocated if it is compatible with multus, but cannot be allocated if it is not compatible.", Label("D00009", "D00010"), func() {
			// Generate daemonset yaml and annotation
			dsObject := common.GenerateExampleDaemonSetYaml(dsName, namespace)
			dsObject.Spec.Template.Annotations = map[string]string{common.MultusDefaultNetwork: fmt.Sprintf("%s/%s", namespace, spiderMultusNadName)}
			GinkgoWriter.Printf("Try to create daemonset: %v/%v \n", namespace, dsName)
			Expect(frame.CreateDaemonSet(dsObject)).NotTo(HaveOccurred())

			GinkgoWriter.Println("multusName has affinity with IPPool and can assign IP")
			Eventually(func() bool {
				podList, err := frame.GetPodListByLabel(dsObject.Spec.Template.Labels)
				if err != nil {
					GinkgoWriter.Printf("failed to get pod list by label, error is %v", err)
					return false
				}
				return frame.CheckPodListRunning(podList)
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

			// Create a set of daemonset again, use the default multus cr, and specify the pool with affinity to other multus cr.
			// The daemonset will fail to create and the event will be as expected.
			var unAffinityDsName = "un-affinit-ds-" + common.GenerateString(10, true)
			unAffinityDsObject := common.GenerateExampleDaemonSetYaml(unAffinityDsName, namespace)
			ippoolAnno := types.AnnoPodIPPoolValue{}
			if frame.Info.IpV4Enabled {
				ippoolAnno.IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				ippoolAnno.IPv6Pools = []string{v6PoolName}
			}
			ippoolAnnoMarshal, err := json.Marshal(ippoolAnno)
			Expect(err).NotTo(HaveOccurred())
			unAffinityDsObject.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: string(ippoolAnnoMarshal)}
			GinkgoWriter.Printf("Try to create daemonset: %v/%v \n", namespace, unAffinityDsName)
			Expect(frame.CreateDaemonSet(unAffinityDsObject)).NotTo(HaveOccurred())

			var podList *corev1.PodList
			Eventually(func() bool {
				podList, err = frame.GetPodListByLabel(unAffinityDsObject.Spec.Template.Labels)
				if err != nil {
					GinkgoWriter.Printf("failed to get pod list by label, error is %v", err)
					return false
				}
				if len(podList.Items) != len(frame.Info.KindNodeList) {
					return false
				}
				return true
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

			var unmacthedMultusCRString string
			if frame.Info.IpV6Enabled && !frame.Info.IpV4Enabled {
				unmacthedMultusCRString = fmt.Sprintf("interface eth0 IPPool %v specified multusName [%v] unmacthed multusCR", v6PoolName, spiderMultusNadName)
			} else {
				unmacthedMultusCRString = fmt.Sprintf("interface eth0 IPPool %v specified multusName [%v] unmacthed multusCR", v4PoolName, spiderMultusNadName)
			}

			GinkgoWriter.Println("multusName has no affinity with IPPool and cannot assign IP")
			for _, pod := range podList.Items {
				ctx, cancel := context.WithTimeout(context.Background(), common.EventOccurTimeout)
				defer cancel()
				err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, pod.Namespace, unmacthedMultusCRString)
				Expect(err).NotTo(HaveOccurred(), "Failedto get event, error is %v", err)
			}
		})
	})

	// This use case is used to test the affinity of nodeName in ippool.
	// The use case design is as follows:
	// 1. Create ippool cr and associate the node through ippool.spec.nodeName
	// 2. Create a set of daemonSet
	// 3. The Pod is scheduled to a node that has affinity with IPPool, and the Pod can run normally.
	// 4. The Pod is scheduled to a node that does not have affinity with IPPool, and the Pod cannot run.
	Context("Test nodeName affinity", func() {
		var namespace, v4PoolName, v6PoolName, dsName string
		var iPv4PoolObj, iPv6PoolObj *spiderpoolv2beta1.SpiderIPPool
		var v4SubnetName, v6SubnetName string
		var v4SubnetObject, v6SubnetObject *spiderpoolv2beta1.SpiderSubnet
		var matchedNode *corev1.Node
		var podAnnoMarshalString string

		BeforeEach(func() {
			dsName = "ds-" + common.GenerateString(10, true)
			namespace = "ns" + tools.RandomName()

			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			GinkgoWriter.Printf("create namespace %v. \n", namespace)
			Expect(err).NotTo(HaveOccurred())

			nodeList, err := frame.GetNodeList()
			Expect(err).NotTo(HaveOccurred())
			matchedNode = &nodeList.Items[0]
			GinkgoWriter.Printf("Set the node affinity with the IP pool to %+v", matchedNode.Name)

			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			podIppoolsAnno := types.AnnoPodIPPoolValue{}
			if frame.Info.IpV4Enabled {
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(1)
				// Associate IPPool with nodeName
				iPv4PoolObj.Spec.NodeName = []string{matchedNode.Name}
				if frame.Info.SpiderSubnetEnabled {
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, len(frame.Info.KindNodeList))
					Expect(v4SubnetObject).NotTo(BeNil())
					Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, len(frame.Info.KindNodeList))
				} else {
					err = common.CreateIppool(frame, iPv4PoolObj)
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to create v4 Pool %v \n", v4PoolName)
				podIppoolsAnno.IPv4Pools = []string{v4PoolName}
			}

			if frame.Info.IpV6Enabled {
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(len(frame.Info.KindNodeList))
				// Associate IPPool with nodeName
				iPv6PoolObj.Spec.NodeName = []string{matchedNode.Name}
				if frame.Info.SpiderSubnetEnabled {
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, len(frame.Info.KindNodeList))
					Expect(v6SubnetObject).NotTo(BeNil())
					Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, len(frame.Info.KindNodeList))
				} else {
					err = common.CreateIppool(frame, iPv6PoolObj)
				}
				Expect(err).NotTo(HaveOccurred(), "Failed to create v6 Pool %v \n", v6PoolName)
				podIppoolsAnno.IPv6Pools = []string{v6PoolName}

			}
			podAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			podAnnoMarshalString = string(podAnnoMarshal)

			DeferCleanup(func() {
				if CurrentSpecReport().Failed() {
					GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
					return
				}
				GinkgoWriter.Printf("delete namespace %v. \n", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())

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

		It("If IPPool has affinity with nodeName, it can allocate IP, but if it has no affinity, it cannot allocate IP.", Label("D00011", "D00012"), func() {
			// Generate daemonset yaml with annotation
			dsObject := common.GenerateExampleDaemonSetYaml(dsName, namespace)
			dsObject.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podAnnoMarshalString}
			GinkgoWriter.Printf("Try to create daemonset: %v/%v \n", namespace, dsName)
			err := frame.CreateDaemonSet(dsObject)
			Expect(err).NotTo(HaveOccurred())

			var podList *corev1.PodList
			// Wait for Pod replicas on all nodes to be pulled up.
			Eventually(func() bool {
				podList, err = frame.GetPodListByLabel(dsObject.Spec.Template.Labels)
				if err != nil {
					GinkgoWriter.Printf("failed to get pod list by label, error is %v", err)
					return false
				}
				return len(podList.Items) == len(frame.Info.KindNodeList)
			}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

			for _, pod := range podList.Items {
				if pod.Spec.NodeName == matchedNode.Name {
					// Schedule to a node that has affinity with IPPool, the Pod can run normally.
					Eventually(func() bool {
						podOnMatchedNode, err := frame.GetPod(pod.Name, pod.Namespace)
						if err != nil {
							GinkgoWriter.Printf("failed to get pod, error is %v", err)
							return false
						}
						return frame.CheckPodListRunning(&corev1.PodList{Items: []corev1.Pod{*podOnMatchedNode}})
					}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())
				} else {
					// Scheduled to a node that has no affinity with IPPool, the Pod cannot run.
					var unmacthedNodeNameString string
					if frame.Info.IpV6Enabled && !frame.Info.IpV4Enabled {
						unmacthedNodeNameString = fmt.Sprintf("unmatched Node name of IPPool %v", v6PoolName)
					} else {
						unmacthedNodeNameString = fmt.Sprintf("unmatched Node name of IPPool %v", v4PoolName)
					}
					ctx, cancel := context.WithTimeout(context.Background(), common.EventOccurTimeout)
					defer cancel()
					err := frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, namespace, unmacthedNodeNameString)
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})
	})
})
