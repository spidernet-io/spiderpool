// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package annotation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test annotation", Label("annotation"), func() {
	var nsName, podName, nic string

	BeforeEach(func() {
		// Init test info and create namespace
		nic = "eth0"
		podName = "pod" + tools.RandomName()
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("Create namespace %v \n", nsName)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, time.Second*10)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// Clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
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
		Expect(pod.Annotations[annotationKeyName]).To(Equal(podYaml.Annotations[annotationKeyName]))

		// When an annotation has an invalid field or value, the Pod will fail to run.
		ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel1()
		err = frame.WaitExceptEventOccurred(ctx1, common.PodEventKind, podName, nsName, common.CNIFailedToSetUpNetwork)
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
				"ipv4pools": ["IPamNotExistedPool"],
				"ipv6pools": ["IPamNotExistedPool"]
			}`),
		Entry("fail to run a pod with non-existed ippool NIC values", Label("A00003"), Pending, pkgconstant.AnnoPodIPPool,
			`{
				"interface": "IPamNotExistedNIC",
				"ipv4pools": ["default-v4-ippool"],
				"ipv6pools": ["default-v6-ippool"]
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
				"ipv4pools": ["default-v4-ippool"],
				"ipv6pools": ["default-v6-ippool"]
			}`),
		Entry("fail to run a pod with non-existed ippools v4、v6 values", Label("A00003"), pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "eth0",
				"ipv4pools": ["IPamNotExistedPool"],
				"ipv6pools": ["IPamNotExistedPool"],
				"defaultRoute": true
			 }]`),
		Entry("fail to run a pod with non-existed ippools NIC values", Label("A00003"), Pending, pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "IPamNotExistedNIC",
				"ipv4pools": ["default-v4-ippool"],
				"ipv6pools": ["default-v6-ippool"],
				"defaultRoute": true
			  }]`),
		Entry("fail to run a pod with non-existed ippools defaultRoute values", Label("A00003"), pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "eth0",
				"ipv4pools": ["default-v4-ippool"],
				"ipv6pools": ["default-v6-ippool"],
				"defaultRoute": IPamErrRouteBool
			   }]`),
		Entry("fail to run a pod with non-existed ippools NIC key", Label("A00003"), pkgconstant.AnnoPodIPPools,
			`[{
				"IPamNotExistedNICKey": "eth0",
				"ipv4pools": ["default-v4-ippool"],
				"ipv6pools": ["default-v6-ippool"],
				"defaultRoute": true
				}]`),
		Entry("fail to run a pod with non-existed ippools v4、v6 key", Label("A00003"), pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "eth0",
				"IPamNotExistedPoolKey": ["default-v4-ippool"],
				"IPamNotExistedPoolKey": ["default-v6-ippool"],
				"defaultRoute": true
				}]`),
		Entry("fail to run a pod with non-existed ippools defaultRoute key", Label("A00003"), Pending, pkgconstant.AnnoPodIPPools,
			`[{
				"interface": "eth0",
				"ipv4pools": ["default-v4-ippool"],
				"ipv6pools": ["default-v6-ippool"],
				"IPamNotExistedRouteKey": true
				}]`),
	)

	It("it fails to run a pod with different VLAN for ipv4 and ipv6 ippool", Label("A00001"), func() {
		var (
			v4PoolName, v6PoolName   string
			iPv4PoolObj, iPv6PoolObj *spiderpool.SpiderIPPool
			ipv4vlan, ipv6vlan       = new(types.Vlan), new(types.Vlan)
		)
		// Different VLAN for ipv4 and ipv6 Pool
		*ipv4vlan = 10
		*ipv6vlan = 20

		// The case relies on a Dual-stack
		if !frame.Info.IpV6Enabled || !frame.Info.IpV4Enabled {
			Skip("Test conditions（Dual-stack）are not met")
		}

		// Create IPv4Pool and IPv6Pool
		v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(200)
		iPv4PoolObj.Spec.Vlan = ipv4vlan
		GinkgoWriter.Printf("try to create ipv4pool: %v \n", v4PoolName)
		err := common.CreateIppool(frame, iPv4PoolObj)
		Expect(err).NotTo(HaveOccurred(), "failed to create ipv4pool %v \n", v4PoolName)

		v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(200)
		iPv6PoolObj.Spec.Vlan = ipv6vlan
		GinkgoWriter.Printf("try to create ipv6pool: %v \n", v6PoolName)
		err = common.CreateIppool(frame, iPv6PoolObj)
		Expect(err).NotTo(HaveOccurred(), "failed to create ipv6pool %v \n", v6PoolName)

		// Generate IPPool annotations string
		podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, nic, []string{v4PoolName}, []string{v6PoolName})

		// Generate Pod yaml and add IPPool annotations to it
		GinkgoWriter.Printf("try to create pod %v/%v with annotation %v=%v \n", nsName, podName, pkgconstant.AnnoPodIPPool, podIppoolAnnoStr)
		podYaml := common.GenerateExamplePodYaml(podName, nsName)
		podYaml.Annotations = map[string]string{pkgconstant.AnnoPodIPPool: podIppoolAnnoStr}
		Expect(frame.CreatePod(podYaml)).NotTo(HaveOccurred())

		// It fails to run a pod with different VLAN for ipv4 and ipv6 ippool
		ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel1()
		GinkgoWriter.Printf("different VLAN for ipv4 and ipv6 ippool with fail to run pod %v/%v \n", nsName, podName)
		err = frame.WaitExceptEventOccurred(ctx1, common.PodEventKind, podName, nsName, common.CNIFailedToSetUpNetwork)
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
		var podIppoolAnnoStr, podIppoolsAnnoStr string
		var v4PoolNameList, v6PoolNameList []string
		var cleanGateway bool
		var err error
		BeforeEach(func() {
			cleanGateway = false
			if frame.Info.IpV4Enabled {
				v4PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, 200, true)
				Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool")
			}
			if frame.Info.IpV6Enabled {
				v6PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, 200, false)
				Expect(err).NotTo(HaveOccurred(), "Failed to create v6 pool")
			}
			GinkgoWriter.Printf("Successful creation of v4Pool %v，v6Pool %v", v4PoolNameList, v6PoolNameList)

			DeferCleanup(func() {
				GinkgoWriter.Printf("Try to delete v4PoolList %v, v6PoolList %v \n", v4PoolNameList, v6PoolNameList)
				if frame.Info.IpV4Enabled {
					for _, pool := range v4PoolNameList {
						Expect(common.DeleteIPPoolByName(frame, pool)).NotTo(HaveOccurred())
					}
				}
				if frame.Info.IpV6Enabled {
					for _, pool := range v6PoolNameList {
						Expect(common.DeleteIPPoolByName(frame, pool)).NotTo(HaveOccurred())
					}
				}
			})
		})

		It(`the "ippools" annotation has the higher priority over the "ippool" annotation`, Label("A00005"), func() {
			// Generate IPPool annotation string
			podIppoolAnnoStr = common.GeneratePodIPPoolAnnotations(frame, nic, ClusterDefaultV4IppoolList, ClusterDefaultV6IppoolList)

			// Generate IPPools annotation string
			podIppoolsAnnoStr = common.GeneratePodIPPoolsAnnotations(frame, nic, cleanGateway, v4PoolNameList, v6PoolNameList)

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
		It(`A00008: Successfully run an annotated multi-container pod
			E00007: Succeed to run a pod with long yaml for ipv4, ipv6 and dual-stack case`, Label("A00008", "E00007"), func() {
			var containerName = "cn" + tools.RandomName()
			var annotationKeyName = "test-long-yaml-" + tools.RandomName()
			var annotationLength int = 200
			var containerNum int = 2

			// Generate IPPool annotation string
			podIppoolAnnoStr = common.GeneratePodIPPoolAnnotations(frame, nic, v4PoolNameList, v6PoolNameList)

			// Generate a pod yaml with multiple containers and long annotations
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			containerObject := podYaml.Spec.Containers[0]
			containerObject.Name = containerName
			podYaml.Spec.Containers = append(podYaml.Spec.Containers, containerObject)
			podYaml.Annotations = map[string]string{pkgconstant.AnnoPodIPPool: podIppoolAnnoStr,
				annotationKeyName: common.GenerateString(annotationLength, false)}
			Expect(podYaml).NotTo(BeNil())

			pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, time.Second*30)
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
			Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, &corev1.PodList{Items: []corev1.Pod{*pod}}, time.Minute)).To(Succeed())
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

			It(`the pod annotation has the highest priority over namespace and global default ippool`,
				Label("A00004", "smoke"), func() {
					var newV4PoolNameList, newV6PoolNameList []string

					if frame.Info.IpV4Enabled {
						newV4PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, 200, true)
						Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool")
						v4PoolNameList = append(v4PoolNameList, newV4PoolNameList...)
					}
					if frame.Info.IpV6Enabled {
						newV6PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, 200, false)
						Expect(err).NotTo(HaveOccurred(), "Failed to create v6 pool")
						v6PoolNameList = append(v6PoolNameList, newV6PoolNameList...)
					}

					// Generate Pod.IPPool annotations string
					podIppoolAnnoStr = common.GeneratePodIPPoolAnnotations(frame, nic, newV4PoolNameList, newV6PoolNameList)

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

			It("Spiderpool will successively try to allocate IP in the order of the elements in the IPPool array until the first allocation succeeds or all fail",
				Label("A00007"), func() {
					var v4PoolNameList1, v4PoolNameList2, v6PoolNameList1, v6PoolNameList2 []string
					var deployName = "deploy" + tools.RandomName()

					var (
						podOriginialNum int = 1
						podScaleupNum   int = 2
						ippoolIpNum     int = 1
					)

					// Create two ippools to be used as backup ippools
					if frame.Info.IpV4Enabled {
						v4PoolNameList1, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ippoolIpNum, true)
						Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool %v", v4PoolNameList1)
						v4PoolNameList2, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ippoolIpNum, true)
						Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool %v", v4PoolNameList2)
						v4PoolNameList = append(append(v4PoolNameList, v4PoolNameList1...), v4PoolNameList2...)
					}
					if frame.Info.IpV6Enabled {
						v6PoolNameList1, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ippoolIpNum, false)
						Expect(err).NotTo(HaveOccurred(), "Failed to create v6 pool %v", v6PoolNameList1)
						v6PoolNameList2, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ippoolIpNum, false)
						Expect(err).NotTo(HaveOccurred(), "Failed to create v6 pool %v", v6PoolNameList2)
						v6PoolNameList = append(append(v6PoolNameList, v6PoolNameList1...), v6PoolNameList2...)
					}

					// Create Deployment with types.AnnoPodIPPoolValue and The Pods IP is recorded in the IPPool.
					deploy := common.CreateDeployWithPodAnnoation(frame, deployName, nsName, podOriginialNum, nic, append(v4PoolNameList1, v4PoolNameList2...), append(v6PoolNameList1, v6PoolNameList2...))
					podList := common.CheckPodIpReadyByLabel(frame, deploy.Spec.Template.Labels, v4PoolNameList, v6PoolNameList)

					// Wait for new Pod to be created and expect its ip to be in the next pool in the array
					deploy, err = frame.ScaleDeployment(deploy, int32(podScaleupNum))
					Expect(err).NotTo(HaveOccurred(), "Failed to scale deployment")
					ctx2, cancel2 := context.WithTimeout(context.Background(), time.Minute)
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
					Expect(frame.DeleteDeploymentUntilFinish(deployName, nsName, time.Minute)).To(Succeed())
					GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", nsName, deployName)
					Expect(common.WaitIPReclaimedFinish(frame, append(v4PoolNameList1, v4PoolNameList2...), append(v6PoolNameList1, v6PoolNameList2...), scalePodList, time.Minute)).To(Succeed())
					GinkgoWriter.Println("Pod IP is successfully released")
				})

			It(`the namespace annotation has precedence over global default ippool`,
				Label("A00006", "smoke"), func() {
					// Generate a pod yaml with namespace annotations
					podYaml := common.GenerateExamplePodYaml(podName, nsName)
					Expect(podYaml).NotTo(BeNil())

					// The namespace annotation has precedence over global default ippool
					checkAnnotationPriority(podYaml, podName, nsName, v4PoolNameList, v6PoolNameList)
				})
		})
	})
	It("succeeded to running pod after added valid route field", Label("A00002"), func() {
		var ipv4Gw, ipv6Gw string
		v4Dst := "0.0.0.0/0"
		v6Dst := "::/0"
		annoPodRouteValue := new(types.AnnoPodRoutesValue)
		annoPodIPPoolValue := types.AnnoPodIPPoolValue{}

		var v4PoolName, v6PoolName string
		var v4Pool, v6Pool *spiderpool.SpiderIPPool

		// create ippool
		if frame.Info.IpV4Enabled {
			GinkgoWriter.Println("create v4 ippool")
			v4PoolName, v4Pool = common.GenerateExampleIpv4poolObject(1)
			Expect(v4Pool).NotTo(BeNil())
			Expect(v4PoolName).NotTo(BeEmpty())
			Expect(common.CreateIppool(frame, v4Pool)).To(Succeed(), "failed to create v4 ippool %v\n", v4PoolName)

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
			Expect(common.CreateIppool(frame, v6Pool)).To(Succeed(), "failed to create v6 ippool %v\n", v6PoolName)

			subnet := v6Pool.Spec.Subnet
			ipv6Gw = strings.Split(subnet, "/")[0] + "1"
			*annoPodRouteValue = append(*annoPodRouteValue, types.AnnoRouteItem{
				Dst: v6Dst,
				Gw:  ipv6Gw,
			})
			annoPodIPPoolValue.IPv6Pools = []string{v6PoolName}
		}

		annoPodRouteB, err := json.Marshal(*annoPodRouteValue)
		Expect(err).NotTo(HaveOccurred(), "failed to marshal annoPodRouteValue, error: %v\n", err)
		annoPodRoutStr := string(annoPodRouteB)

		annoPodIPPoolB, err := json.Marshal(annoPodIPPoolValue)
		Expect(err).NotTo(HaveOccurred(), "failed to marshal annoPodIPPoolValue, error: %v\n", err)
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
		ctxCreate, cancelCreate := context.WithTimeout(context.Background(), time.Minute)
		defer cancelCreate()
		pod, err := frame.WaitPodStarted(podName, nsName, ctxCreate)
		Expect(err).NotTo(HaveOccurred(), "timeout to wait pod %v/%v started\n", nsName, podName)
		Expect(pod).NotTo(BeNil())

		// check whether the route is effective
		GinkgoWriter.Println("check whether the route is effective")
		if frame.Info.IpV4Enabled {
			command := fmt.Sprintf("ip r | grep 'default via %s'", ipv4Gw)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			_, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v\n", command)
		}
		if frame.Info.IpV6Enabled {
			command := fmt.Sprintf("ip -6 r | grep 'default via %s'", ipv6Gw)
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			_, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v\n", command)
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
})

func checkAnnotationPriority(podYaml *corev1.Pod, podName, nsName string, v4PoolNameList, v6PoolNameList []string) {

	pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, time.Second*30)
	GinkgoWriter.Printf("pod %v/%v: podIPv4: %v, podIPv6: %v \n", nsName, podName, podIPv4, podIPv6)

	// Check Pod IP recorded in IPPool
	podlist := &corev1.PodList{
		Items: []corev1.Pod{*pod},
	}
	ok, _, _, e := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podlist)
	Expect(e).NotTo(HaveOccurred())
	Expect(ok).To(BeTrue())

	// Try to delete Pod
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := frame.DeletePodUntilFinish(podName, nsName, ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %v/%v \n", nsName, podName)
	GinkgoWriter.Printf("Succeeded to delete pod %v/%v \n", nsName, podName)

	// Check if the Pod IP in IPPool reclaimed normally
	Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podlist, time.Minute)).To(Succeed())
	GinkgoWriter.Println("Pod ip successfully released")
}
