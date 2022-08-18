// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package annotation_test

import (
	"context"
	"encoding/json"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test annotation", Label("annotation"), func() {
	var nsName, podName string

	BeforeEach(func() {
		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, time.Second*10)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)
		// init test name
		podName = "pod" + tools.RandomName()
		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	DescribeTable("invalid annotations table", func(annotationKeyName string, annotationKeyValue string) {
		// try to create pod
		GinkgoWriter.Printf("try to create pod %v/%v with annotation %v=%v \n", nsName, podName, annotationKeyName, annotationKeyValue)
		podYaml := common.GenerateExamplePodYaml(podName, nsName)
		podYaml.Annotations = map[string]string{annotationKeyName: annotationKeyValue}
		Expect(podYaml).NotTo(BeNil())
		err := frame.CreatePod(podYaml)
		Expect(err).NotTo(HaveOccurred())
		// check annotation
		pod, err := frame.GetPod(podName, nsName)
		Expect(err).NotTo(HaveOccurred())
		Expect(pod.Annotations[annotationKeyName]).To(Equal(podYaml.Annotations[annotationKeyName]))
		ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
		defer cancel1()
		// fail to run pod
		GinkgoWriter.Printf("Invalid input fail to run pod %v/%v \n", nsName, podName)
		err = frame.WaitExceptEventOccurred(ctx1, common.PodEventKind, podName, nsName, common.CNIFailedToSetUpNetwork)
		Expect(err).NotTo(HaveOccurred(), "failed to get event  %v/%v %v\n", nsName, podName, common.CNIFailedToSetUpNetwork)
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
	Context("different VLAN for ipv4 and ipv6 ippool", func() {
		var v4PoolName, v6PoolName, nic, podAnnoStr string
		var iPv4PoolObj, iPv6PoolObj *spiderpool.SpiderIPPool
		var ipv4vlan = new(types.Vlan)
		var ipv6vlan = new(types.Vlan)

		BeforeEach(func() {
			nic = "eth0"
			*ipv4vlan = 10
			*ipv6vlan = 20
			// create ipv4pool
			if frame.Info.IpV4Enabled {
				// Generate v4PoolName and ipv4pool object
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(200)
				iPv4PoolObj.Spec.Vlan = ipv4vlan
				GinkgoWriter.Printf("try to create ipv4pool: %v/%v \n", v4PoolName, iPv4PoolObj)
				err := common.CreateIppool(frame, iPv4PoolObj)
				Expect(err).NotTo(HaveOccurred(), "fail to create ipv4pool %v \n", v4PoolName)
			}
			// create ipv6pool
			if frame.Info.IpV6Enabled {
				// Generate v6PoolName and ipv6pool object
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(200)
				iPv6PoolObj.Spec.Vlan = ipv6vlan
				GinkgoWriter.Printf("try to create ipv6pool: %v/%v \n", v6PoolName, iPv6PoolObj)
				err := common.CreateIppool(frame, iPv6PoolObj)
				Expect(err).NotTo(HaveOccurred(), "fail to create ipv6pool %v \n", v6PoolName)
			}
			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Printf("try to delete ipv4pool %v \n", v4PoolName)
					err := common.DeleteIPPoolByName(frame, v4PoolName)
					Expect(err).NotTo(HaveOccurred())
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Printf("try to delete ipv6pool %v \n", v6PoolName)
					err := common.DeleteIPPoolByName(frame, v6PoolName)
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
		It("it fails to run a pod with different VLAN for ipv4 and ipv6 ippool", Label("A00001"), func() {
			if !frame.Info.IpV6Enabled || !frame.Info.IpV4Enabled {
				Skip("Test conditions（Dual-stack） are not met")
			}
			podAnno := types.AnnoPodIPPoolValue{
				NIC:       &nic,
				IPv4Pools: []string{v4PoolName},
				IPv6Pools: []string{v6PoolName},
			}
			b, e1 := json.Marshal(podAnno)
			Expect(e1).NotTo(HaveOccurred())
			podAnnoStr = string(b)

			// try to create pod
			GinkgoWriter.Printf("try to create pod %v/%v with annotation %v=%v \n", nsName, podName, pkgconstant.AnnoPodIPPool, podAnnoStr)
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			podYaml.Annotations = map[string]string{pkgconstant.AnnoPodIPPool: podAnnoStr}
			Expect(podYaml).NotTo(BeNil())
			err := frame.CreatePod(podYaml)
			Expect(err).NotTo(HaveOccurred())

			// fail to run pod
			ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel1()
			GinkgoWriter.Printf("different VLAN for ipv4 and ipv6 ippool with fail to run pod %v/%v \n", nsName, podName)
			err = frame.WaitExceptEventOccurred(ctx1, common.PodEventKind, podName, nsName, common.CNIFailedToSetUpNetwork)
			Expect(err).NotTo(HaveOccurred(), "fail to get event %v/%v = %v\n", nsName, podName, common.CNIFailedToSetUpNetwork)
			pod, err := frame.GetPod(podName, nsName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Status.Phase).To(Equal(corev1.PodPending))

			// try to delete pod
			GinkgoWriter.Printf("try to delete pod %v/%v \n", nsName, podName)
			err = frame.DeletePod(podName, nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", nsName, podName)
		})
	})
	Context("annotation priority", func() {
		var nic, podIppoolAnnoStr, podIppoolsAnnoStr string
		var v4PoolNameList, v6PoolNameList []string
		var cleanGateway bool
		var err error
		BeforeEach(func() {
			nic = "eth0"
			cleanGateway = false
			if frame.Info.IpV4Enabled {
				v4PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, 200, true)
				Expect(err).NotTo(HaveOccurred(), "Failed to create v4 pool")
			}
			if frame.Info.IpV6Enabled {
				v6PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, 200, false)
				Expect(err).NotTo(HaveOccurred(), "Failed to create v6 pool")
			}
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

		It(`the "ippools" annotation has the higher priority over the "ippool" annotation`, Label("A00005"), func() {
			// ippool annotation
			podIppoolAnno := types.AnnoPodIPPoolValue{
				NIC: &nic,
			}
			if frame.Info.IpV4Enabled {
				podIppoolAnno.IPv4Pools = ClusterDefaultV4IppoolList
			}
			if frame.Info.IpV6Enabled {
				podIppoolAnno.IPv6Pools = ClusterDefaultV6IppoolList
			}
			b, err := json.Marshal(podIppoolAnno)
			Expect(err).NotTo(HaveOccurred())
			podIppoolAnnoStr = string(b)

			// ippools annotation
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC:          nic,
					CleanGateway: cleanGateway,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = v4PoolNameList
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = v6PoolNameList
			}
			b, err = json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			podIppoolsAnnoStr = string(b)

			// Generate Pod Yaml %v with ippool annotations and ippools annotations
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			Expect(podYaml).NotTo(BeNil())
			podYaml.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPool:  podIppoolAnnoStr,
				pkgconstant.AnnoPodIPPools: podIppoolsAnnoStr,
			}
			GinkgoWriter.Printf("Generate Pod Yaml %v with ippool annotations and ippools annotations", podYaml)

			// The "ippools" annotation has a higher priority than the "ippool" annotation.
			checkAnnotationPriority(podYaml, podName, nsName, v4PoolNameList, v6PoolNameList)
		})
		It(`Successfully run an annotated multi-container pod`, Label("A00008"), func() {
			var containerName = "cn" + tools.RandomName()
			// ippool annotation
			podIppoolAnno := types.AnnoPodIPPoolValue{
				NIC: &nic,
			}
			if frame.Info.IpV4Enabled {
				podIppoolAnno.IPv4Pools = v4PoolNameList
			}
			if frame.Info.IpV6Enabled {
				podIppoolAnno.IPv6Pools = v6PoolNameList
			}
			b, err := json.Marshal(podIppoolAnno)
			Expect(err).NotTo(HaveOccurred())
			podIppoolAnnoStr = string(b)

			// Generate multi-container pod yaml with ippool annotations
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			containerObject := podYaml.Spec.Containers[0]
			containerObject.Name = containerName
			podYaml.Spec.Containers = append(podYaml.Spec.Containers, containerObject)
			podYaml.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPool: podIppoolAnnoStr,
			}
			Expect(podYaml).NotTo(BeNil())

			pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, time.Second*30)
			GinkgoWriter.Printf("pod %v/%v: podIPv4: %v, podIPv6: %v \n", nsName, podName, podIPv4, podIPv6)

			// Check multi-container Pod
			Expect((len(pod.Status.ContainerStatuses))).Should(Equal(2))

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
			err = frame.DeletePodUntilFinish(podName, nsName, ctx)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %v/%v \n", nsName, podName)
			GinkgoWriter.Printf("Successful deletion of pods %v/%v \n", nsName, podName)

			// check if the pod ip in ippool reclaimed normally
			Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podlist, time.Minute)).To(Succeed())
			GinkgoWriter.Println("Pod IP successfully released")

		})

		Context("Because the following cases are annotated about namespaces, the namespaces are annotated first", func() {
			var v4NamespaceIppoolAnnoStr, v6NamespaceIppoolAnnoStr string

			BeforeEach(func() {

				// get namespace object
				namespaceObject, err := frame.GetNamespace(nsName)
				Expect(err).NotTo(HaveOccurred())

				// namespace annotation
				namespaceObject.Annotations = make(map[string]string)
				if frame.Info.IpV4Enabled {
					v4IppoolAnnoValue := types.AnnoNSDefautlV4PoolValue{}
					b, err := json.Marshal(append(v4IppoolAnnoValue, v4PoolNameList...))
					Expect(err).NotTo(HaveOccurred())
					v4NamespaceIppoolAnnoStr = string(b)
					namespaceObject.Annotations[pkgconstant.AnnoNSDefautlV4Pool] = v4NamespaceIppoolAnnoStr
				}
				if frame.Info.IpV6Enabled {
					v6IppoolAnnoValue := types.AnnoNSDefautlV6PoolValue{}
					b, err := json.Marshal(append(v6IppoolAnnoValue, v6PoolNameList...))
					Expect(err).NotTo(HaveOccurred())
					v6NamespaceIppoolAnnoStr = string(b)
					namespaceObject.Annotations[pkgconstant.AnnoNSDefautlV6Pool] = v6NamespaceIppoolAnnoStr
				}
				GinkgoWriter.Printf("Generate namespace objects: %v with namespace annotations \n", namespaceObject)

				// update namespace object
				err = frame.UpdateResource(namespaceObject)
				Expect(err).NotTo(HaveOccurred())
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

					// pod annotations（Ippool）
					podAnno := types.AnnoPodIPPoolValue{
						NIC: &nic,
					}
					if frame.Info.IpV4Enabled {
						podAnno.IPv4Pools = newV4PoolNameList
					}
					if frame.Info.IpV6Enabled {
						podAnno.IPv6Pools = newV6PoolNameList
					}
					b, e := json.Marshal(podAnno)
					Expect(e).NotTo(HaveOccurred())
					podIppoolAnnoStr = string(b)

					// Generate pod yaml with pod annotations(Ippool)
					podYaml := common.GenerateExamplePodYaml(podName, nsName)
					Expect(podYaml).NotTo(BeNil())
					podYaml.Annotations = map[string]string{
						pkgconstant.AnnoPodIPPool: podIppoolAnnoStr,
					}
					GinkgoWriter.Printf("Generate Pod Yaml %v with pod annotations(Ippool)", podYaml)

					// The pod annotations have the highest priority over namespaces and the global default ippool.
					checkAnnotationPriority(podYaml, podName, nsName, newV4PoolNameList, newV6PoolNameList)
				})

			It("Spiderpool will successively try to allocate IP in the order of the elements in the IPPool array until the first allocation succeeds or all fail",
				Label("A00007"), func() {
					var v4PoolNameList1, v4PoolNameList2, v6PoolNameList1, v6PoolNameList2 []string
					var deployName = "deploy" + tools.RandomName()
					var (
						podOriginialNum int = 2
						podScaleupNum   int = 4
						ippoolIpNum     int = 2
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

					// Generate Pod annotations(IPPool)
					podAnno := types.AnnoPodIPPoolValue{
						NIC: &nic,
					}
					if frame.Info.IpV4Enabled {
						podAnno.IPv4Pools = append(v4PoolNameList1, v4PoolNameList2...)
					}
					if frame.Info.IpV6Enabled {
						podAnno.IPv6Pools = append(v6PoolNameList1, v6PoolNameList2...)
					}
					b, e := json.Marshal(podAnno)
					Expect(e).NotTo(HaveOccurred())
					podIppoolAnnoStr = string(b)

					// Generate Deployment Yaml and type in annotation
					deployYaml := common.GenerateExampleDeploymentYaml(deployName, nsName, int32(podOriginialNum))
					deployYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}
					Expect(deployYaml).NotTo(BeNil())

					// Create Deployment/Pod until ready
					deploy, err := frame.CreateDeploymentUntilReady(deployYaml, time.Minute)
					Expect(err).NotTo(HaveOccurred())

					// Get Pod list
					podList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
					Expect(err).NotTo(HaveOccurred())

					// Check Pod IP record in IPPool
					ok, _, _, err := common.CheckPodIpRecordInIppool(frame, v4PoolNameList1, v6PoolNameList1, podList)
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).To(BeTrue())

					// Wait for new Pod to be created and expect its ip to be in the next pool in the array
					deploy, err = frame.ScaleDeployment(deploy, int32(podScaleupNum))
					Expect(err).NotTo(HaveOccurred(), "Failed to scale deployment")
					ctx2, cancel2 := context.WithTimeout(context.Background(), time.Minute)
					defer cancel2()
					err = frame.WaitPodListRunning(deploy.Spec.Selector.MatchLabels, podScaleupNum, ctx2)
					Expect(err).NotTo(HaveOccurred())

					// Get the scaled pod list
					scalePodList, err := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
					Expect(err).NotTo(HaveOccurred())
					pods := common.GetAdditionalPods(podList, scalePodList)
					Expect(len(pods)).To(Equal(podScaleupNum - podOriginialNum))
					addPodList := &corev1.PodList{
						Items: pods,
					}

					// Check the Pod's IP record backup IPPool
					ok, _, _, err = common.CheckPodIpRecordInIppool(frame, v4PoolNameList2, v6PoolNameList2, addPodList)
					Expect(err).NotTo(HaveOccurred())
					Expect(ok).To(BeTrue())

					//Try to delete Deployment
					Expect(frame.DeleteDeploymentUntilFinish(deployName, nsName, time.Minute)).To(Succeed())
					GinkgoWriter.Printf("Succeeded to delete deployment %v/%v \n", nsName, deployName)

					// Check that the Pod IP in the IPPool is reclaimed properly
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
})

func checkAnnotationPriority(podYaml *corev1.Pod, podName, nsName string, v4PoolNameList, v6PoolNameList []string) {

	pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, time.Second*30)
	GinkgoWriter.Printf("pod %v/%v: podIPv4: %v, podIPv6: %v \n", nsName, podName, podIPv4, podIPv6)

	// check pod ip recorded in ippool
	podlist := &corev1.PodList{
		Items: []corev1.Pod{*pod},
	}
	ok, _, _, e := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, podlist)
	Expect(e).NotTo(HaveOccurred())
	Expect(ok).To(BeTrue())

	// try to delete pod
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := frame.DeletePodUntilFinish(podName, nsName, ctx)
	Expect(err).NotTo(HaveOccurred(), "Failed to delete pod %v/%v \n", nsName, podName)
	GinkgoWriter.Printf("Succeeded to delete pod %v/%v \n", nsName, podName)

	// check if the pod ip in ippool reclaimed normally
	Expect(common.WaitIPReclaimedFinish(frame, v4PoolNameList, v6PoolNameList, podlist, time.Minute)).To(Succeed())
	GinkgoWriter.Println("Pod ip successfully released")
}
