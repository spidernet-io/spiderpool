// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippoolcr_test

import (
	"context"
	"encoding/json"
	"fmt"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
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
		var v4PoolObj, v4PoolObj1, v6PoolObj, v6PoolObj1, v4Pool, v6Pool *spiderpoolv1.SpiderIPPool
		var v4PoolNameList, v6PoolNameList []string
		var disable = new(bool)

		BeforeEach(func() {
			if frame.Info.IpV4Enabled {
				v4PoolName, v4PoolObj = common.GenerateExampleIpv4poolObject(5)
				Expect(v4PoolObj.Spec.IPs).NotTo(BeNil())
				// create ipv4 pool
				GinkgoWriter.Printf("Create v4 ippool %v\n", v4PoolObj.Name)
				Expect(common.CreateIppool(frame, v4PoolObj)).To(Succeed())
				GinkgoWriter.Printf("Succeeded to create v4 ippool %v \n", v4PoolObj.Name)
				v4PoolNameList = append(v4PoolNameList, v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, v6PoolObj = common.GenerateExampleIpv6poolObject(5)
				Expect(v6PoolObj.Spec.IPs).NotTo(BeNil())
				// create ipv6 pool
				GinkgoWriter.Printf("Create v6 ippool %v\n", v6PoolObj.Name)
				Expect(common.CreateIppool(frame, v6PoolObj)).To(Succeed())
				GinkgoWriter.Printf("Succeeded to create v6 ippool %v \n", v6PoolObj.Name)
				v6PoolNameList = append(v6PoolNameList, v6PoolName)
			}
			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					GinkgoWriter.Printf("Delete ippool %v\n", v4PoolName)
					Expect(common.DeleteIPPoolUntilFinish(frame, v4PoolName, ctx)).To(Succeed())
				}
				if frame.Info.IpV6Enabled {
					ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
					defer cancel()
					GinkgoWriter.Printf("Delete ippool %v\n", v6PoolName)
					Expect(common.DeleteIPPoolUntilFinish(frame, v6PoolName, ctx)).To(Succeed())
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

		It("add a route with `routes` and `gateway` fields in the ippool spec", Label("D00002", "D00003"), func() {
			podName := "pod" + tools.RandomName()
			annoPodIPPool := types.AnnoPodIPPoolValue{}
			var v4Gateway, v6Gateway, v4Dst, v6Dst, v4Via, v6Via string
			var v4InvalidGateway, v6InvalidGateway string

			num1 := common.GenerateRandomNumber(255)
			num2 := common.GenerateRandomNumber(255)
			num3 := common.GenerateRandomNumber(255)
			num4 := common.GenerateRandomNumber(255)
			v4InvalidGateway = fmt.Sprintf("%s.%s.%s.%s", num1, num2, num3, num4)
			v4Dst = fmt.Sprintf("%s.%s.0.0/16", num3, num4)

			num1 = common.GenerateRandomNumber(9999)
			num2 = common.GenerateRandomNumber(9999)
			num3 = common.GenerateRandomNumber(9999)
			num4 = common.GenerateRandomNumber(9999)
			v6InvalidGateway = fmt.Sprintf("%s:%s:%s::%s", num1, num2, num3, num4)
			v6Dst = fmt.Sprintf("%s:%s::/32", num3, num4)

			if frame.Info.IpV4Enabled {
				annoPodIPPool.IPv4Pools = []string{v4PoolName}
				v4Gateway = strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "1"
				v4Via = strings.Split(v4PoolObj.Spec.Subnet, "0/")[0] + "254"
			}
			if frame.Info.IpV6Enabled {
				annoPodIPPool.IPv6Pools = []string{v6PoolName}
				v6Gateway = strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "1"
				v6Via = strings.Split(v6PoolObj.Spec.Subnet, "/")[0] + "fe"
			}

			// update ippool
			if frame.Info.IpV4Enabled {
				By("update v4 pool: invalid `gateway` and valid `route`")
				// get ipv4 pool
				v4Pool = common.GetIppoolByName(frame, v4PoolName)
				Expect(v4Pool).NotTo(BeNil())

				v4Pool.Spec.Gateway = &v4InvalidGateway
				route := spiderpoolv1.Route{
					Dst: v4Dst,
					Gw:  v4Via,
				}
				v4Pool.Spec.Routes = []spiderpoolv1.Route{route}
				// update v4 ippool
				Expect(frame.UpdateResource(v4Pool)).NotTo(Succeed(), "error: we expect failed to update v4 ippool: %v with invalid gateway: %v, and valid route: %+v\n", v4PoolName, v4InvalidGateway, route)

				By("update v4 pool: valid `gateway` and invalid `route`")
				// get ipv4 pool
				v4Pool = common.GetIppoolByName(frame, v4PoolName)
				Expect(v4Pool).NotTo(BeNil())

				v4Pool.Spec.Gateway = &v4Gateway
				route = spiderpoolv1.Route{
					Dst: v4Dst,
					Gw:  v4InvalidGateway,
				}
				v4Pool.Spec.Routes = []spiderpoolv1.Route{route}
				// update v4 ippool
				Expect(frame.UpdateResource(v4Pool)).NotTo(Succeed(), "error: we expect failed to update v4 ippool: %v with valid gateway: %v, and invalid route: %+v\n", v4PoolName, v4InvalidGateway, route)

				By("update v4 pool: valid `gateway` and `route`")
				// get ipv4 pool
				v4Pool = common.GetIppoolByName(frame, v4PoolName)
				Expect(v4Pool).NotTo(BeNil())

				v4Pool.Spec.Gateway = &v4Gateway
				route = spiderpoolv1.Route{
					Dst: v4Dst,
					Gw:  v4Via,
				}
				v4Pool.Spec.Routes = []spiderpoolv1.Route{route}
				// update v4 ippool
				Expect(frame.UpdateResource(v4Pool)).To(Succeed(), "failed to update v4 ippool: %v with valid gateway: %v, and route: %+v\n", v4PoolName, v4Gateway, route)
			}
			if frame.Info.IpV6Enabled {
				By("update v6 pool: invalid `gateway` and valid `route`")
				// get ipv6 pool
				v6Pool = common.GetIppoolByName(frame, v6PoolName)
				Expect(v6Pool).NotTo(BeNil())

				v6PoolObj.Spec.Gateway = &v6InvalidGateway
				route := spiderpoolv1.Route{
					Dst: v6Dst,
					Gw:  v6Via,
				}
				v6PoolObj.Spec.Routes = []spiderpoolv1.Route{route}
				// update v6 ippool
				Expect(frame.UpdateResource(v6PoolObj)).NotTo(Succeed(), "error: we expect failed to update v6 ippool: %v with invalid gateway: %v, and valid route: %+v\n", v6PoolName, v6InvalidGateway, route)

				By("update v6 pool: valid `gateway` and invalid `route`")
				// get ipv6 pool
				v6Pool = common.GetIppoolByName(frame, v6PoolName)
				Expect(v6Pool).NotTo(BeNil())

				v6PoolObj.Spec.Gateway = &v6Gateway
				route = spiderpoolv1.Route{
					Dst: v6Dst,
					Gw:  v6InvalidGateway,
				}
				v6PoolObj.Spec.Routes = []spiderpoolv1.Route{route}
				// update v6 ippool
				Expect(frame.UpdateResource(v6PoolObj)).NotTo(Succeed(), "error: we expect failed to update v6 ippool: %v with valid gateway: %v, and invalid route: %+v\n", v6PoolName, v6InvalidGateway, route)

				By("update v6 pool: valid `gateway` and `route`")
				// get ipv6 pool
				v6Pool = common.GetIppoolByName(frame, v6PoolName)
				Expect(v6Pool).NotTo(BeNil())

				v6Pool.Spec.Gateway = &v6Gateway
				route = spiderpoolv1.Route{
					Dst: v6Dst,
					Gw:  v6Via,
				}
				v6Pool.Spec.Routes = []spiderpoolv1.Route{route}
				// update v6 ippool
				Expect(frame.UpdateResource(v6Pool)).To(Succeed(), "failed to update v6 ippool: %v with valid gateway: %v, and route: %+v\n", v6PoolName, v6Gateway, route)
			}

			// create pod
			common.CreatePodWithAnnoPodIPPool(frame, podName, nsName, annoPodIPPool)

			// check whether the route information is effective
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
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			Expect(frame.DeletePodUntilFinish(podName, nsName, ctx)).To(Succeed())
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
