// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package annotation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test annotation", Label("annotation"), func() {
	var nsName, podName string
	var v4SubnetName, v6SubnetName, globalV4PoolName, globalV6PoolName string
	var v4SubnetObject, v6SubnetObject *spiderpool.SpiderSubnet
	var globalDefaultV4IpoolList, globalDefaultV6IpoolList []string
	var globalv4pool, globalv6pool *spiderpool.SpiderIPPool

	BeforeEach(func() {
		// Adapt to the default subnet, create a new pool as a public pool
		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()
		if frame.Info.IpV4Enabled {
			globalV4PoolName, globalv4pool = common.GenerateExampleIpv4poolObject(10)
			if frame.Info.SpiderSubnetEnabled {
				GinkgoWriter.Printf("Create v4 subnet %v and v4 pool %v \n", v4SubnetName, globalV4PoolName)
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(100)
				Expect(v4SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
				err := common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, globalv4pool, 3)
				Expect(err).NotTo(HaveOccurred())
			} else {
				err := common.CreateIppool(frame, globalv4pool)
				Expect(err).NotTo(HaveOccurred())
			}
			globalDefaultV4IpoolList = append(globalDefaultV4IpoolList, globalV4PoolName)
		}
		if frame.Info.IpV6Enabled {
			globalV6PoolName, globalv6pool = common.GenerateExampleIpv6poolObject(10)
			if frame.Info.SpiderSubnetEnabled {
				GinkgoWriter.Printf("Create v6 subnet %v and v6 pool %v \n", v6SubnetName, globalV6PoolName)
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(100)
				Expect(v6SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
				err := common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, globalv6pool, 3)
				Expect(err).NotTo(HaveOccurred())
			} else {
				err := common.CreateIppool(frame, globalv6pool)
				Expect(err).NotTo(HaveOccurred())
			}
			globalDefaultV6IpoolList = append(globalDefaultV6IpoolList, globalV6PoolName)
		}

		// Init test info and create namespace
		podName = "pod" + tools.RandomName()
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("Create namespace %v \n", nsName)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// Clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			Expect(frame.DeleteNamespace(nsName)).NotTo(HaveOccurred())
			GinkgoWriter.Printf("delete v4subnet %v v4 pool %v, v6subnet %v v6 pool %v\n", v4SubnetName, globalV4PoolName, v6SubnetName, globalV6PoolName)
			if frame.Info.IpV4Enabled {
				Expect(common.DeleteIPPoolByName(frame, globalV4PoolName)).NotTo(HaveOccurred())
				if frame.Info.SpiderSubnetEnabled {
					Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
				}
				globalDefaultV4IpoolList = []string{}
			}
			if frame.Info.IpV6Enabled {
				Expect(common.DeleteIPPoolByName(frame, globalV6PoolName)).NotTo(HaveOccurred())
				if frame.Info.SpiderSubnetEnabled {
					Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
				}
				globalDefaultV6IpoolList = []string{}
			}
		})
	})

	DescribeTable("invalid annotations table", func(annotationKeyName string, annotationKeyValue string) {
		// Generate example Pod yaml and create Pod
		GinkgoWriter.Printf("try to create pod %v/%v with annotation %v=%v \n", nsName, podName, annotationKeyName, annotationKeyValue)
		podYaml := common.GenerateExamplePodYaml(podName, nsName)
		podYaml.Annotations = map[string]string{annotationKeyName: annotationKeyValue}
		Expect(podYaml).NotTo(BeNil())
		err := frame.CreatePod(podYaml)
		Expect(err).NotTo(HaveOccurred())

		// Get pods and check annotation
		pod, err := frame.GetPod(podName, nsName)
		Expect(err).NotTo(HaveOccurred())

		// When an annotation has an invalid field or value, the Pod will fail to run.
		ctx1, cancel1 := context.WithTimeout(context.Background(), common.EventOccurTimeout)
		defer cancel1()
		err = frame.WaitExceptEventOccurred(ctx1, common.OwnerPod, podName, nsName, common.CNIFailedToSetUpNetwork)
		Expect(err).NotTo(HaveOccurred(), "failed to get event  %v/%v %v\n", nsName, podName, common.CNIFailedToSetUpNetwork)
		GinkgoWriter.Printf("The annotation has an invalid field or value and the Pod %v/%v fails to run. \n", nsName, podName)
		Expect(pod.Status.Phase).To(Equal(corev1.PodPending))

		// try to delete pod
		GinkgoWriter.Printf("try to delete pod %v/%v \n", nsName, podName)
		err = frame.DeletePod(podName, nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", nsName, podName)
	},
		// TODO(tao.yang), routes、dns、status unrealized;
		Entry("fail to run a pod with non-existed ippool v4、v6 values", Label("A00003"), pkgconstant.AnnoPodIPPool,
			`{
				"interface": "eth0",
				"ipv4": ["IPamNotExistedPool"],
				"ipv6": ["IPamNotExistedPool"]
			}`),
		Entry("fail to run a pod with non-existed ippool NIC values", Label("A00003"), Pending, pkgconstant.AnnoPodIPPool,
			`{
				"interface": "IPamNotExistedNIC",
				"ipv4": ["default-v4-ippool"],
				"ipv4": ["default-v6-ippool"]
			}`),
		Entry("fail to run a pod with non-existed ippool v4、v6 key", Label("A00003"), pkgconstant.AnnoPodIPPool,
			`{
				"interface": "eth0",
				"IPamNotExistedPoolKey": ["default-v4-ippool"],
				"IPamNotExistedPoolKey": ["default-v6-ippool"]
			}`),
		Entry("fail to run a pod with non-existed ippool NIC key", Label("A00003"), Pending, pkgconstant.AnnoPodIPPool,
			`{
				"IPamNotExistedNICKey": "eth0",
				"ipv4": ["default-v4-ippool"],
				"ipv6": ["default-v6-ippool"]
			}`),
		Entry("fail to run a pod with non-existed ippools v4、v6 values", Label("A00003"), pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "eth0",
				"ipv4": ["IPamNotExistedPool"],
				"ipv6": ["IPamNotExistedPool"],
				"cleanGateway": true
			 }]`),
		Entry("fail to run a pod with non-existed ippools NIC values", Label("A00003"), Pending, pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "IPamNotExistedNIC",
				"ipv4": ["default-v4-ippool"],
				"ipv6": ["default-v6-ippool"],
				"cleanGateway": true
			  }]`),
		Entry("fail to run a pod with non-existed ippools defaultRoute values", Label("A00003"), pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "eth0",
				"ipv4": ["default-v4-ippool"],
				"ipv6": ["default-v6-ippool"],
				"cleanGateway": IPamErrRouteBool
			   }]`),
		Entry("fail to run a pod with non-existed ippools NIC key", Label("A00003"), pkgconstant.AnnoPodIPPools,
			`[{
				"IPamNotExistedNICKey": "eth0",
				"ipv4": ["default-v4-ippool"],
				"ipv6": ["default-v6-ippool"],
				"cleanGateway": true
				}]`),
		Entry("fail to run a pod with non-existed ippools v4、v6 key", Label("A00003"), pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "eth0",
				"IPamNotExistedPoolKey": ["default-v4-ippool"],
				"IPamNotExistedPoolKey": ["default-v6-ippool"],
				"cleanGateway": true
				}]`),
		Entry("fail to run a pod with non-existed ippools defaultRoute key", Label("A00003"), Pending, pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "eth0",
				"ipv4": ["default-v4-ippool"],
				"ipv6": ["default-v6-ippool"],
				"IPamNotExistedRouteKey": true
				}]`),
	)

	It("it fails to run a pod with different VLAN for ipv4 and ipv6 ippool", Label("A00001"), Pending, func() {
		var (
			v4PoolName, v6PoolName   string
			iPv4PoolObj, iPv6PoolObj *spiderpool.SpiderIPPool
			ipv4vlan, ipv6vlan       = new(types.Vlan), new(types.Vlan)
			err                      error
			ipNum                    int = 2
		)
		// Different VLAN for ipv4 and ipv6 Pool
		*ipv4vlan = 10
		*ipv6vlan = 20

		// The case relies on a Dual-stack
		if !frame.Info.IpV6Enabled || !frame.Info.IpV4Enabled {
			Skip("Test conditions（Dual-stack）are not met")
		}

		// Create IPv4Pool and IPv6Pool
		v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(ipNum)
		iPv4PoolObj.Spec.Vlan = ipv4vlan
		GinkgoWriter.Printf("try to create ipv4pool: %v \n", v4PoolName)
		if frame.Info.SpiderSubnetEnabled {
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, ipNum)
		} else {
			err = common.CreateIppool(frame, iPv4PoolObj)
		}
		Expect(err).NotTo(HaveOccurred(), "failed to create ipv4pool %v \n", v4PoolName)

		v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(ipNum)
		iPv6PoolObj.Spec.Vlan = ipv6vlan
		GinkgoWriter.Printf("try to create ipv6pool: %v \n", v6PoolName)
		if frame.Info.SpiderSubnetEnabled {
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, ipNum)
		} else {
			err = common.CreateIppool(frame, iPv6PoolObj)
		}
		Expect(err).NotTo(HaveOccurred(), "failed to create ipv6pool %v \n", v6PoolName)

		// Generate IPPool annotations string
		podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, []string{v4PoolName}, []string{v6PoolName})

		// Generate Pod yaml and add IPPool annotations to it
		GinkgoWriter.Printf("try to create pod %v/%v with annotation %v=%v \n", nsName, podName, pkgconstant.AnnoPodIPPool, podIppoolAnnoStr)
		podYaml := common.GenerateExamplePodYaml(podName, nsName)
		podYaml.Annotations = map[string]string{pkgconstant.AnnoPodIPPool: podIppoolAnnoStr}
		Expect(frame.CreatePod(podYaml)).NotTo(HaveOccurred())

		// It fails to run a pod with different VLAN for ipv4 and ipv6 ippool
		ctx1, cancel1 := context.WithTimeout(context.Background(), common.EventOccurTimeout)
		defer cancel1()
		GinkgoWriter.Printf("different VLAN for ipv4 and ipv6 ippool with fail to run pod %v/%v \n", nsName, podName)
		err = frame.WaitExceptEventOccurred(ctx1, common.OwnerPod, podName, nsName, common.CNIFailedToSetUpNetwork)
		Expect(err).NotTo(HaveOccurred(), "Failedto get event %v/%v = %v\n", nsName, podName, common.CNIFailedToSetUpNetwork)
		pod, err := frame.GetPod(podName, nsName)
		Expect(err).NotTo(HaveOccurred())
		Expect(pod.Status.Phase).To(Equal(corev1.PodPending))

		// Cleaning up the test env
		Expect(frame.DeletePod(podName, nsName)).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", nsName, podName)
		GinkgoWriter.Printf("Successful deletion of pods %v/%v \n", nsName, podName)
		Expect(common.DeleteIPPoolByName(frame, v4PoolName)).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Successful deletion of ipv4pool %v \n", v4PoolName)
		Expect(common.DeleteIPPoolByName(frame, v6PoolName)).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Successful deletion of ipv6pool %v \n", v6PoolName)
	})

	Context("annotation priority", func() {
		var v4PoolName, v6PoolName, podIppoolAnnoStr, podIppoolsAnnoStr string
		var iPv4PoolObj, iPv6PoolObj *spiderpool.SpiderIPPool
		var v4PoolNameList, v6PoolNameList []string
		var cleanGateway bool
		var err error
		var ipNum int = 10

		BeforeEach(func() {
			cleanGateway = false
			if frame.Info.IpV4Enabled {
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(ipNum)
				if frame.Info.SpiderSubnetEnabled {
					ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
					defer cancel()
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, ipNum)
				} else {
					err = common.CreateIppool(frame, iPv4PoolObj)
				}
				v4PoolNameList = append(v4PoolNameList, v4PoolName)
				Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool %v \n", v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(ipNum)
				if frame.Info.SpiderSubnetEnabled {
					ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
					defer cancel()
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, ipNum)
				} else {
					err = common.CreateIppool(frame, iPv6PoolObj)
				}
				v6PoolNameList = append(v6PoolNameList, v6PoolName)
				Expect(err).NotTo(HaveOccurred(), "Failed to create v6 pool %v", v6PoolName)
			}
			GinkgoWriter.Printf("Successful creation of v4Pool %v，v6Pool %v. \n", v4PoolNameList, v6PoolNameList)

			DeferCleanup(func() {
				GinkgoWriter.Printf("Try to delete v4Pool %v, v6Pool %v. \n", v4PoolNameList, v6PoolNameList)
				if frame.Info.IpV4Enabled {
					for _, pool := range v4PoolNameList {
						Expect(common.DeleteIPPoolByName(frame, pool)).NotTo(HaveOccurred())
					}
					v4PoolNameList = []string{}
				}
				if frame.Info.IpV6Enabled {
					for _, pool := range v6PoolNameList {
						Expect(common.DeleteIPPoolByName(frame, pool)).NotTo(HaveOccurred())
					}
					v6PoolNameList = []string{}
				}
			})
		})

		It(`the "ippools" annotation has the higher priority over the "ippool" annotation`, Label("A00005"), func() {
			// Generate IPPool annotation string
			podIppoolAnnoStr = common.GeneratePodIPPoolAnnotations(frame, common.NIC1, globalDefaultV4IpoolList, globalDefaultV6IpoolList)
			// Generate IPPools annotation string
			podIppoolsAnnoStr = common.GeneratePodIPPoolsAnnotations(frame, common.NIC1, cleanGateway, v4PoolNameList, v6PoolNameList)

			// Generate Pod Yaml with IPPool annotations and IPPools annotations
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			podYaml.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPool:  podIppoolAnnoStr,
				pkgconstant.AnnoPodIPPools: podIppoolsAnnoStr,
			}
			Expect(podYaml).NotTo(BeNil())
			GinkgoWriter.Printf("Successful to generate Pod Yaml with IPPool annotations and IPPools annotations")

			// The "ippools" annotation has a higher priority than the "ippool" annotation.
			checkAnnotationPriority(podYaml, podName, nsName, v4PoolNameList, v6PoolNameList)
		})

		It(`A00008: Successfully run an annotated multi-container pod, E00007: Succeed to run a pod with long yaml for ipv4, ipv6 and dual-stack case`,
			Label("A00008", "E00007"), func() {
				var containerName = "cn" + tools.RandomName()
				var annotationKeyName = "test-long-yaml-" + tools.RandomName()
				var annotationLength int = 200
				var containerNum int = 2

				// Generate IPPool annotation string
				podIppoolAnnoStr = common.GeneratePodIPPoolAnnotations(frame, common.NIC1, v4PoolNameList, v6PoolNameList)

				// Generate a pod yaml with multiple containers and long annotations
				podYaml := common.GenerateExamplePodYaml(podName, nsName)
				containerObject := podYaml.Spec.Containers[0]
				containerObject.Name = containerName
				podYaml.Spec.Containers = append(podYaml.Spec.Containers, containerObject)
				podYaml.Annotations = map[string]string{pkgconstant.AnnoPodIPPool: podIppoolAnnoStr,
					annotationKeyName: common.GenerateString(annotationLength, false)}
				Expect(podYaml).NotTo(BeNil())

				pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, common.PodStartTimeout)
				GinkgoWriter.Printf("Pod %v/%v: podIPv4: %v, podIPv6: %v \n", nsName, podName, podIPv4, podIPv6)

				// A00008: Successfully run an annotated multi-container pod
				// Check multi-container Pod Number
				Expect((len(pod.Status.ContainerStatuses))).Should(Equal(containerNum))

				// E00007: Succeed to run a pod with long yaml for ipv4, ipv6 and dual-stack case
				// Check that the long yaml information is correct.
				Expect(pod.Annotations[annotationKeyName]).To(Equal(podYaml.Annotations[annotationKeyName]))
				Expect(len(pod.Annotations[annotationKeyName])).To(Equal(annotationLength))

				// Check Pod IP record in IPPool
				ok, _, _, e := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, &corev1.PodList{Items: []corev1.Pod{*pod}})
				Expect(e).NotTo(HaveOccurred())
				Expect(ok).To(BeTrue())

				// Delete the Pod and check that the Pod IP in the IPPool is correctly reclaimed.
				Expect(frame.DeletePod(podName, nsName)).NotTo(HaveOccurred(), "Failed to delete Pod %v/%v \n", nsName, podName)
				GinkgoWriter.Printf("Successful deletion of pods %v/%v \n", nsName, podName)
				Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, &corev1.PodList{Items: []corev1.Pod{*pod}}, common.IPReclaimTimeout)).To(Succeed())
				GinkgoWriter.Printf("The Pod %v/%v IP in the IPPool was reclaimed correctly \n", nsName, podName)
			})

		Context("About namespace annotations", func() {
			BeforeEach(func() {
				// Get namespace object and generate namespace annotation
				namespaceObject, err := frame.GetNamespace(nsName)
				Expect(err).NotTo(HaveOccurred())
				namespaceObject.Annotations = make(map[string]string)
				if frame.Info.IpV4Enabled {
					v4IppoolAnnoValue := types.AnnoNSDefautlV4PoolValue{}
					common.SetNamespaceIppoolAnnotation(v4IppoolAnnoValue, namespaceObject, v4PoolNameList, pkgconstant.AnnoNSDefautlV4Pool)
				}
				if frame.Info.IpV6Enabled {
					v6IppoolAnnoValue := types.AnnoNSDefautlV6PoolValue{}
					common.SetNamespaceIppoolAnnotation(v6IppoolAnnoValue, namespaceObject, v6PoolNameList, pkgconstant.AnnoNSDefautlV6Pool)
				}
				GinkgoWriter.Printf("Generate namespace objects: %v with namespace annotations \n", namespaceObject)

				// Update the namespace with the generated namespace object with annotation
				Expect(frame.UpdateResource(namespaceObject)).NotTo(HaveOccurred())
				GinkgoWriter.Printf("Succeeded to update namespace: %v object \n", nsName)
			})

			It(`the pod annotation has the highest priority over namespace and global default ippool`, Label("A00004", "smoke"), func() {
				var newV4PoolNameList, newV6PoolNameList []string

				if frame.Info.IpV4Enabled {
					if frame.Info.SpiderSubnetEnabled {
						v4PoolName, v4Pool := common.GenerateExampleIpv4poolObject(ipNum)
						Expect(v4Pool).NotTo(BeNil())
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4Pool, ipNum)
						newV4PoolNameList = append(newV4PoolNameList, v4PoolName)
					} else {
						newV4PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ipNum, true)
					}
					Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool %v,error is %v", newV4PoolNameList, err)
				}
				if frame.Info.IpV6Enabled {
					if frame.Info.SpiderSubnetEnabled {
						v6PoolName, v6Pool := common.GenerateExampleIpv6poolObject(ipNum)
						Expect(v6Pool).NotTo(BeNil())
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, v6Pool, ipNum)
						newV6PoolNameList = append(newV6PoolNameList, v6PoolName)
					} else {
						newV6PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ipNum, false)
					}
					Expect(err).NotTo(HaveOccurred(), "Failed to create v6 pool %v,error is %v", newV6PoolNameList, err)
				}

				// Generate Pod.IPPool annotations string
				podIppoolAnnoStr = common.GeneratePodIPPoolAnnotations(frame, common.NIC1, newV4PoolNameList, newV6PoolNameList)

				// Generate Pod Yaml
				podYaml := common.GenerateExamplePodYaml(podName, nsName)
				Expect(podYaml).NotTo(BeNil())
				podYaml.Annotations = map[string]string{
					pkgconstant.AnnoPodIPPool: podIppoolAnnoStr,
				}
				GinkgoWriter.Printf("Generate Pod Yaml %v with Pod IPPool annotations", podYaml)

				// The Pod annotations have the highest priority over namespaces and the global default ippool.
				checkAnnotationPriority(podYaml, podName, nsName, newV4PoolNameList, newV6PoolNameList)
			})

			It(`the namespace annotation has precedence over global default ippool`, Label("A00006", "smoke"), func() {
				// Generate a pod yaml with namespace annotations
				if frame.Info.SpiderSubnetEnabled {
					Skip("The subnet function is enabled, the namespace annotation has a lower priority than the default subnet")
				}
				podYaml := common.GenerateExamplePodYaml(podName, nsName)
				Expect(podYaml).NotTo(BeNil())
				// The namespace annotation has precedence over global default ippool
				checkAnnotationPriority(podYaml, podName, nsName, v4PoolNameList, v6PoolNameList)
			})
		})
	})

	Context("Spiderpool will successively try to allocate IP in the order of the elements in the IPPool array until the first allocation succeeds or all fail", func() {
		var v4PoolNameList1, v6PoolNameList1, v4PoolNameList2, v6PoolNameList2 []string
		var err error
		var deployName string
		var (
			podOriginialNum int = 1
			podScaleupNum   int = 2
			ippoolIpNum     int = 1
		)
		BeforeEach(func() {
			deployName = "deploy" + tools.RandomName()
			// Create one ippool to be used as backup ippool
			if frame.Info.IpV4Enabled {
				if frame.Info.SpiderSubnetEnabled {
					ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
					defer cancel()
					v4PoolName1, iPv4PoolObj1 := common.GenerateExampleIpv4poolObject(ippoolIpNum)
					Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj1, ippoolIpNum)).NotTo(HaveOccurred(), "Failed to create v4 pool")
					v4PoolNameList1 = append(v4PoolNameList1, v4PoolName1)
					v4PoolName2, iPv4PoolObj2 := common.GenerateExampleIpv4poolObject(ippoolIpNum)
					Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj2, ippoolIpNum)).NotTo(HaveOccurred(), "Failed to create v4 pool")
					v4PoolNameList2 = append(v4PoolNameList2, v4PoolName2)
				} else {
					v4PoolNameList1, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ippoolIpNum, true)
					Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool")
					v4PoolNameList2, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ippoolIpNum, true)
					Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool")
				}
			}
			if frame.Info.IpV6Enabled {
				if frame.Info.SpiderSubnetEnabled {
					ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
					defer cancel()
					v6PoolName1, iPv6PoolObj1 := common.GenerateExampleIpv6poolObject(ippoolIpNum)
					Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj1, ippoolIpNum)).NotTo(HaveOccurred(), "Failed to create v6 pool")
					v6PoolNameList1 = append(v6PoolNameList1, v6PoolName1)
					v6PoolName2, iPv6PoolObj2 := common.GenerateExampleIpv6poolObject(ippoolIpNum)
					Expect(common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj2, ippoolIpNum)).NotTo(HaveOccurred(), "Failed to create v6 pool")
					v6PoolNameList2 = append(v6PoolNameList2, v6PoolName2)
				} else {
					v6PoolNameList1, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ippoolIpNum, false)
					Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool")
					v6PoolNameList2, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ippoolIpNum, false)
					Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool")
				}
			}
			GinkgoWriter.Printf("Successful creation of v4Pool %v,v6Pool %v \n", append(v4PoolNameList1, v4PoolNameList2...), append(v6PoolNameList1, v6PoolNameList2...))

			DeferCleanup(func() {
				GinkgoWriter.Printf("Try to delete v4PoolList %v, v6PoolList %v \n", append(v4PoolNameList1, v4PoolNameList2...), append(v6PoolNameList1, v6PoolNameList2...))
				if frame.Info.IpV4Enabled {
					for _, pool := range append(v4PoolNameList1, v4PoolNameList2...) {
						Expect(common.DeleteIPPoolByName(frame, pool)).NotTo(HaveOccurred())
					}
				}
				if frame.Info.IpV6Enabled {
					for _, pool := range append(v6PoolNameList1, v6PoolNameList2...) {
						Expect(common.DeleteIPPoolByName(frame, pool)).NotTo(HaveOccurred())
					}
				}
			})
		})
		It("Spiderpool will successively try to allocate IP in the order of the elements in the IPPool array until the first allocation succeeds or all fail", Serial,
			Label("A00007"), func() {

				// Create Deployment with types.AnnoPodIPPoolValue and The Pods IP is recorded in the IPPool.
				deploy := common.CreateDeployWithPodAnnoation(frame, deployName, nsName, podOriginialNum, common.NIC1, append(v4PoolNameList1, v4PoolNameList2...), append(v6PoolNameList1, v6PoolNameList2...))
				podList := common.CheckPodIpReadyByLabel(frame, deploy.Spec.Template.Labels, v4PoolNameList1, v6PoolNameList1)

				// Wait for new Pod to be created and expect its ip to be in the next pool in the array
				deploy, err = frame.ScaleDeployment(deploy, int32(podScaleupNum))
				Expect(err).NotTo(HaveOccurred(), "Failed to scale deployment")
				ctx2, cancel2 := context.WithTimeout(context.Background(), common.PodReStartTimeout)
				defer cancel2()
				err = frame.WaitPodListRunning(deploy.Spec.Selector.MatchLabels, podScaleupNum, ctx2)
				Expect(err).NotTo(HaveOccurred())

				// Get an extended list of pods
				scalePodList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
				Expect(err).NotTo(HaveOccurred())
				pods := common.GetAdditionalPods(podList, scalePodList)
				Expect(len(pods)).To(Equal(podScaleupNum - podOriginialNum))
				addPodList := &corev1.PodList{
					Items: pods,
				}

				// Check the Pod's IP record backup IPPool
				ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList2, v6PoolNameList2, addPodList)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeTrue())

				// Delete Deployment and check that the Pod IP in the IPPool is reclaimed properly
				Expect(frame.DeleteDeploymentUntilFinish(deployName, nsName, common.ResourceDeleteTimeout)).To(Succeed())
				GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", nsName, deployName)
				Expect(common.WaitIPReclaimedFinish(frame, append(v4PoolNameList1, v4PoolNameList2...), append(v6PoolNameList1, v6PoolNameList2...), scalePodList, common.IPReclaimTimeout)).To(Succeed())
				GinkgoWriter.Println("Pod IP is successfully released")
			})
	})
	It("succeeded to running pod after added valid route field", Label("A00002"), func() {
		var v4PoolName, v6PoolName, ipv4Gw, ipv6Gw string
		var v4Pool, v6Pool *spiderpool.SpiderIPPool
		var err error

		v4Dst := "0.0.0.0/0"
		v6Dst := "::/0"
		annoPodRouteValue := new(types.AnnoPodRoutesValue)
		annoPodIPPoolValue := types.AnnoPodIPPoolValue{}

		// create ippool
		if frame.Info.IpV4Enabled {
			GinkgoWriter.Println("create v4 ippool")
			v4PoolName, v4Pool = common.GenerateExampleIpv4poolObject(1)
			Expect(v4Pool).NotTo(BeNil())
			Expect(v4PoolName).NotTo(BeEmpty())
			if frame.Info.SpiderSubnetEnabled {
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4Pool, 1)
			} else {
				err = common.CreateIppool(frame, v4Pool)
			}
			Expect(err).To(Succeed(), "failed to create v4 ippool %v ,err is %v\n", v4PoolName, err)

			subnet := v4Pool.Spec.Subnet
			ipv4Gw = strings.Split(subnet, "0/")[0] + "1"
			*annoPodRouteValue = append(*annoPodRouteValue, types.AnnoRouteItem{
				Dst: v4Dst,
				Gw:  ipv4Gw,
			})
			annoPodIPPoolValue.IPv4Pools = []string{v4PoolName}
		}
		if frame.Info.IpV6Enabled {
			GinkgoWriter.Println("create v6 ippool")
			v6PoolName, v6Pool = common.GenerateExampleIpv6poolObject(1)
			Expect(v6Pool).NotTo(BeNil())
			Expect(v6PoolName).NotTo(BeEmpty())
			if frame.Info.SpiderSubnetEnabled {
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, v6Pool, 1)
			} else {
				err = common.CreateIppool(frame, v6Pool)
			}
			Expect(err).To(Succeed(), "failed to create v6 ippool %v ,err is %v\n", v6PoolName, err)

			subnet := v6Pool.Spec.Subnet
			ipv6Gw = strings.Split(subnet, "/")[0] + "1"
			*annoPodRouteValue = append(*annoPodRouteValue, types.AnnoRouteItem{
				Dst: v6Dst,
				Gw:  ipv6Gw,
			})
			annoPodIPPoolValue.IPv6Pools = []string{v6PoolName}
		}

		annoPodRouteB, err := json.Marshal(*annoPodRouteValue)
		Expect(err).NotTo(HaveOccurred(), "failed to marshal annoPodRouteValue, error: %v.\n", err)
		annoPodRoutStr := string(annoPodRouteB)

		annoPodIPPoolB, err := json.Marshal(annoPodIPPoolValue)
		Expect(err).NotTo(HaveOccurred(), "failed to marshal annoPodIPPoolValue, error: %v.\n", err)
		annoPodIPPoolStr := string(annoPodIPPoolB)

		// generate pod yaml
		GinkgoWriter.Println("generate pod yaml")
		podYaml := common.GenerateExamplePodYaml(podName, nsName)
		podYaml.Annotations = map[string]string{
			pkgconstant.AnnoPodRoutes: annoPodRoutStr,
			pkgconstant.AnnoPodIPPool: annoPodIPPoolStr,
		}
		Expect(podYaml).NotTo(BeNil(), "failed to generate pod yaml")
		GinkgoWriter.Printf("succeeded to generate pod yaml: %+v\n", podYaml)

		// create pod
		GinkgoWriter.Printf("create pod %v/%v\n", nsName, podName)
		Expect(frame.CreatePod(podYaml)).To(Succeed(), "failed to create pod %v/%v\n", nsName, podName)
		ctxCreate, cancelCreate := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancelCreate()
		pod, err := frame.WaitPodStarted(podName, nsName, ctxCreate)
		Expect(err).NotTo(HaveOccurred(), "timeout to wait pod %v/%v started\n", nsName, podName)
		Expect(pod).NotTo(BeNil())

		// check whether the route is effective
		GinkgoWriter.Println("check whether the route is effective")
		if frame.Info.IpV4Enabled {
			command := fmt.Sprintf("ip r | grep 'default via %s'", ipv4Gw)
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			_, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v\n", command)
		}
		if frame.Info.IpV6Enabled {
			command := "ip -6 r | grep 'default via'| awk '{print $3}' "
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			out, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v\n", command)
			effectiveIpv6GwStr := strings.TrimSpace(string(out))
			Expect(common.ContrastIpv6ToIntValues(effectiveIpv6GwStr, ipv6Gw)).NotTo(HaveOccurred())
		}

		// delete pod
		GinkgoWriter.Printf("delete pod %v/%v\n", nsName, podName)
		Expect(frame.DeletePod(podName, nsName)).To(Succeed(), "failed to delete pod")

		// Delete IPV4Pool and IPV6Pool
		if frame.Info.IpV4Enabled {
			GinkgoWriter.Printf("delete v4 ippool %v\n", v4PoolName)
			Expect(common.DeleteIPPoolByName(frame, v4PoolName)).To(Succeed())
		}
		if frame.Info.IpV6Enabled {
			GinkgoWriter.Printf("delete v6 ippool %v\n", v6PoolName)
			Expect(common.DeleteIPPoolByName(frame, v6PoolName)).To(Succeed())
		}
	})

	It("Successfully run pods with multi-NIC ippools annotations", Label("A00010"), func() {
		var v4PoolName, v6PoolName string
		var v4Pool, v6Pool *spiderpool.SpiderIPPool
		var err error

		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()
		if frame.Info.IpV4Enabled {
			GinkgoWriter.Println("create v4 ippool")
			v4PoolName, v4Pool = common.GenerateExampleIpv4poolObject(1)
			Expect(v4Pool).NotTo(BeNil())
			if frame.Info.SpiderSubnetEnabled {
				err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4Pool, 1)
			} else {
				err = common.CreateIppool(frame, v4Pool)
			}
			Expect(err).To(Succeed(), "failed to create v4 ippool %v ,err is %v. \n", v4PoolName, err)
		}
		if frame.Info.IpV6Enabled {
			GinkgoWriter.Println("create v6 ippool")
			v6PoolName, v6Pool = common.GenerateExampleIpv6poolObject(1)
			Expect(v6Pool).NotTo(BeNil())
			if frame.Info.SpiderSubnetEnabled {
				err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, v6Pool, 1)
			} else {
				err = common.CreateIppool(frame, v6Pool)
			}
			Expect(err).To(Succeed(), "failed to create v6 ippool %v ,err is %v. \n", v6PoolName, err)
		}

		// set pod annotation for nics
		podIppoolsAnno := types.AnnoPodIPPoolsValue{
			types.AnnoIPPoolItem{
				NIC: common.NIC1,
			}, {
				NIC: common.NIC2,
			},
		}
		if frame.Info.IpV4Enabled {
			podIppoolsAnno[0].IPv4Pools = globalDefaultV4IpoolList
			podIppoolsAnno[1].IPv4Pools = []string{v4PoolName}
		}
		if frame.Info.IpV6Enabled {
			podIppoolsAnno[0].IPv6Pools = globalDefaultV6IpoolList
			podIppoolsAnno[1].IPv6Pools = []string{v6PoolName}
		}
		podIppoolsAnnoMarshal, err := json.Marshal(podIppoolsAnno)
		Expect(err).NotTo(HaveOccurred())
		annoPodIPPoolsStr := string(podIppoolsAnnoMarshal)
		podYaml := common.GenerateExamplePodYaml(podName, nsName)
		podYaml.Annotations = map[string]string{
			pkgconstant.AnnoPodIPPools: annoPodIPPoolsStr,
			common.MultusNetworks:      fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan100),
		}
		Expect(podYaml).NotTo(BeNil())
		GinkgoWriter.Printf("succeeded to generate pod yaml: %+v. \n", podYaml)

		Expect(frame.CreatePod(podYaml)).To(Succeed())
		ctx, cancel = context.WithTimeout(context.Background(), time.Minute*2)
		defer cancel()
		GinkgoWriter.Printf("create a pod %v/%v and wait for ready. \n", nsName, podName)
		pod, err := frame.WaitPodStarted(podName, nsName, ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(pod).NotTo(BeNil())

		GinkgoWriter.Println("Check if multiple NICs are valid.")
		ok, _, _, err := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IpoolList, globalDefaultV6IpoolList, &corev1.PodList{Items: []corev1.Pod{*pod}})
		Expect(ok).NotTo(BeFalse())
		Expect(err).NotTo(HaveOccurred())

		GinkgoWriter.Println("Check for another NIC comment to take effect.")
		if frame.Info.IpV4Enabled {
			command := fmt.Sprintf("ip a show '%s' |grep '%s'", common.NIC2, v4Pool.Spec.IPs[0])
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			errOut, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v, error is %v, %v \n", command, err, string(errOut))
		}
		if frame.Info.IpV6Enabled {
			// The ipv6 address on the network interface will be abbreviated. For example fd00:0c3d::2 becomes fd00:c3d::2.
			// Abbreviate the expected ipv6 address and use it in subsequent assertions.
			ipv6Addr := strings.Replace(v6Pool.Spec.IPs[0], ":0", ":", -1)
			command := fmt.Sprintf("ip -6 a show '%s' | grep '%s'", common.NIC2, ipv6Addr)
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			errOut, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v, error is %v, %v \n", command, err, string(errOut))
		}

		GinkgoWriter.Printf("delete pod %v/%v. \n", nsName, podName)
		Expect(frame.DeletePod(podName, nsName)).To(Succeed())

		// Delete IPV4Pool and IPV6Pool
		if frame.Info.IpV4Enabled {
			GinkgoWriter.Printf("delete v4 ippool %v. \n", v4PoolName)
			Expect(common.DeleteIPPoolByName(frame, v4PoolName)).To(Succeed())
		}
		if frame.Info.IpV6Enabled {
			GinkgoWriter.Printf("delete v6 ippool %v. \n", v6PoolName)
			Expect(common.DeleteIPPoolByName(frame, v6PoolName)).To(Succeed())
		}
	})
})

func checkAnnotationPriority(podYaml *corev1.Pod, podName, nsName string, v4PoolNameList, v6PoolNameList []string) {

	pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, common.PodStartTimeout)
	GinkgoWriter.Printf("pod %v/%v: podIPv4: %v, podIPv6: %v \n", nsName, podName, podIPv4, podIPv6)

	// Check Pod IP recorded in IPPool
	podlist := &corev1.PodList{
		Items: []corev1.Pod{*pod},
	}
	ok, _, _, e := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podlist)
	Expect(e).NotTo(HaveOccurred())
	Expect(ok).To(BeTrue())

	// Try to delete Pod
	ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
	defer cancel()
	err := frame.DeletePodUntilFinish(podName, nsName, ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %v/%v \n", nsName, podName)
	GinkgoWriter.Printf("Succeeded to delete pod %v/%v \n", nsName, podName)

	// Check if the Pod IP in IPPool reclaimed normally
	Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podlist, common.IPReclaimTimeout)).To(Succeed())
	GinkgoWriter.Println("Pod ip successfully released")
}
