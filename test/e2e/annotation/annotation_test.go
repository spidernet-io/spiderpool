// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package annotation_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/utils/pointer"

	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test annotation", Label("annotation"), func() {
	var nsName, podName string
	var v4SubnetName, v6SubnetName, globalV4PoolName, globalV6PoolName string
	var v4SubnetObject, v6SubnetObject *spiderpool.SpiderSubnet
	var globalDefaultV4IpoolList, globalDefaultV6IpoolList []string
	var globalv4pool, globalv6pool *spiderpool.SpiderIPPool

	BeforeEach(func() {
		// Adapt to the default subnet, create a new pool as a public pool
		Eventually(func() error {
			ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
			defer cancel()
			if frame.Info.IpV4Enabled {
				globalV4PoolName, globalv4pool = common.GenerateExampleIpv4poolObject(10)
				if frame.Info.SpiderSubnetEnabled {
					GinkgoWriter.Printf("Create v4 subnet %v and v4 pool %v \n", v4SubnetName, globalV4PoolName)
					v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 100)
					err := common.CreateSubnet(frame, v4SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 Subnet: %v \n", err)
						return err
					}
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, globalv4pool, 3)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool: %v \n", err)
						return err
					}
				} else {
					err := common.CreateIppool(frame, globalv4pool)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool: %v \n", err)
						return err
					}
				}
				globalDefaultV4IpoolList = append(globalDefaultV4IpoolList, globalV4PoolName)
			}
			if frame.Info.IpV6Enabled {
				globalV6PoolName, globalv6pool = common.GenerateExampleIpv6poolObject(10)
				if frame.Info.SpiderSubnetEnabled {
					GinkgoWriter.Printf("Create v6 subnet %v and v6 pool %v \n", v6SubnetName, globalV6PoolName)
					v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 100)
					err := common.CreateSubnet(frame, v6SubnetObject)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 Subnet: %v \n", err)
						return err
					}
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, globalv6pool, 3)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool: %v \n", err)
						return err
					}
				} else {
					err := common.CreateIppool(frame, globalv6pool)
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool: %v \n", err)
						return err
					}
				}
				globalDefaultV6IpoolList = append(globalDefaultV6IpoolList, globalV6PoolName)
			}
			return nil
		}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

		// Init test info and create namespace
		podName = "pod" + tools.RandomName()
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("Create namespace %v \n", nsName)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(nsName, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// Clean test env
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

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

	It("it fails to run a pod with different VLAN for ipv4 and ipv6 ippool", Pending, Label("A00001", "Deprecated"), func() {
		var (
			v4PoolName, v6PoolName   string
			iPv4PoolObj, iPv6PoolObj *spiderpool.SpiderIPPool
			err                      error
			ipNum                    int   = 2
			ipv4Vlan                 int64 = 10
			ipv6Vlan                 int64 = 20
		)

		// The case relies on a Dual-stack
		if !frame.Info.IpV6Enabled || !frame.Info.IpV4Enabled {
			Skip("Test conditions (Dual-stack) are not met")
		}

		// Create IPv4Pool and IPv6Pool
		Eventually(func() error {
			v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(ipNum)
			iPv4PoolObj.Spec.Vlan = pointer.Int64(ipv4Vlan)
			GinkgoWriter.Printf("try to create ipv4pool: %v \n", v4PoolName)
			if frame.Info.SpiderSubnetEnabled {
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, ipNum)
			} else {
				err = common.CreateIppool(frame, iPv4PoolObj)
			}
			if err != nil {
				GinkgoWriter.Printf("Failed to create v4 IPPool: %v \n", err)
				return err
			}

			v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(ipNum)
			iPv6PoolObj.Spec.Vlan = pointer.Int64(ipv6Vlan)
			GinkgoWriter.Printf("try to create ipv6pool: %v \n", v6PoolName)
			if frame.Info.SpiderSubnetEnabled {
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, ipNum)
			} else {
				err = common.CreateIppool(frame, iPv6PoolObj)
			}
			if err != nil {
				GinkgoWriter.Printf("Failed to create v6 IPPool: %v \n", err)
				return err
			}
			return nil
		}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
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
			Eventually(func() error {
				if frame.Info.IpV4Enabled {
					v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(ipNum)
					if frame.Info.SpiderSubnetEnabled {
						ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
						defer cancel()
						err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, ipNum)
					} else {
						err = common.CreateIppool(frame, iPv4PoolObj)
					}
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
						return err
					}
					v4PoolNameList = append(v4PoolNameList, v4PoolName)
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
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
						return err
					}
					v6PoolNameList = append(v6PoolNameList, v6PoolName)
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
			GinkgoWriter.Printf("Successful creation of v4Pool %v, v6Pool %v. \n", v4PoolNameList, v6PoolNameList)

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

				Eventually(func() error {
					if frame.Info.IpV4Enabled {
						if frame.Info.SpiderSubnetEnabled {
							v4PoolName, v4Pool := common.GenerateExampleIpv4poolObject(ipNum)
							ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
							defer cancel()
							err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4Pool, ipNum)
							newV4PoolNameList = append(newV4PoolNameList, v4PoolName)
						} else {
							newV4PoolNameList, err = common.BatchCreateIppoolWithSpecifiedIPNumber(frame, 1, ipNum, true)
						}
						if err != nil {
							GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", newV4PoolNameList, err)
							return err
						}
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
						if err != nil {
							GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", newV6PoolNameList, err)
							return err
						}
					}
					return nil
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())
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
				podYaml := common.GenerateExamplePodYaml(podName, nsName)
				Expect(podYaml).NotTo(BeNil())
				// The namespace annotation has precedence over global default ippool
				checkAnnotationPriority(podYaml, podName, nsName, v4PoolNameList, v6PoolNameList)
			})
		})
	})

	It("succeeded to running pod after added valid route field", Label("A00002"), func() {
		var v4PoolName, v6PoolName, ipv4Dst, ipv6Dst, ipv4Gw, ipv6Gw string
		var v4Pool, v6Pool *spiderpool.SpiderIPPool
		var err error

		annoPodRouteValue := new(types.AnnoPodRoutesValue)
		annoPodIPPoolValue := types.AnnoPodIPPoolValue{}

		// create ippool
		Eventually(func() error {
			if frame.Info.IpV4Enabled {
				GinkgoWriter.Println("create v4 ippool")
				v4PoolName, v4Pool = common.GenerateExampleIpv4poolObject(1)
				if frame.Info.SpiderSubnetEnabled {
					ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
					defer cancel()
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, v4Pool, 1)
				} else {
					err = common.CreateIppool(frame, v4Pool)
				}
				if err != nil {
					GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
					return err
				}

				ipv4Dst = v4Pool.Spec.Subnet
				ipv4Gw = strings.Split(v4Pool.Spec.Subnet, "0/")[0] + "1"
				*annoPodRouteValue = append(*annoPodRouteValue, types.AnnoRouteItem{
					Dst: ipv4Dst,
					Gw:  ipv4Gw,
				})
				annoPodIPPoolValue.IPv4Pools = []string{v4PoolName}
			}
			if frame.Info.IpV6Enabled {
				GinkgoWriter.Println("create v6 ippool")
				v6PoolName, v6Pool = common.GenerateExampleIpv6poolObject(1)
				if frame.Info.SpiderSubnetEnabled {
					ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
					defer cancel()
					err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, v6Pool, 1)
				} else {
					err = common.CreateIppool(frame, v6Pool)
				}
				if err != nil {
					GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
					return err
				}

				ipv6Dst = v6Pool.Spec.Subnet
				ipv6Gw = strings.Split(v6Pool.Spec.Subnet, "/")[0] + "1"
				*annoPodRouteValue = append(*annoPodRouteValue, types.AnnoRouteItem{
					Dst: ipv6Dst,
					Gw:  ipv6Gw,
				})
				annoPodIPPoolValue.IPv6Pools = []string{v6PoolName}
			}
			return nil
		}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

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
			command := fmt.Sprintf("ip r | grep '%s via %s'", ipv4Dst, ipv4Gw)
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			out, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v , error is %v, %v \n", command, err, string(out))
		}
		if frame.Info.IpV6Enabled {
			// The ipv6 address on the network interface will be abbreviated. For example fd00:0c3d::2 becomes fd00:c3d::2.
			// Abbreviate the expected ipv6 address and use it in subsequent assertions.
			_, ipv6Dst, err := net.ParseCIDR(ipv6Dst)
			Expect(err).NotTo(HaveOccurred())
			ipv6Gw := net.ParseIP(ipv6Gw)
			command := fmt.Sprintf("ip -6 r | grep '%s via %s'", ipv6Dst.String(), ipv6Gw.String())
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			out, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v , error is %v, %v \n", command, err, string(out))
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

	Context("run pods with multi-NIC ippools annotations successfully", Label("A00010"), func() {
		var v4PoolName, v6PoolName, newv4SubnetName, newv6SubnetName string
		var v4Pool, v6Pool *spiderpool.SpiderIPPool
		var newv4SubnetObject, newv6SubnetObject *spiderpool.SpiderSubnet
		var err error

		BeforeEach(func() {
			Eventually(func() error {
				ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
				defer cancel()
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Println("create v4 ippool")
					v4PoolName, v4Pool = common.GenerateExampleIpv4poolObject(1)
					if frame.Info.SpiderSubnetEnabled {
						newv4SubnetName, newv4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 100)
						err = common.CreateSubnet(frame, newv4SubnetObject)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v4 Subnet %v: %v \n", newv4SubnetName, err)
							return err
						}
						err = common.CreateIppoolInSpiderSubnet(ctx, frame, newv4SubnetName, v4Pool, 1)
					} else {
						err = common.CreateIppool(frame, v4Pool)
					}
					if err != nil {
						GinkgoWriter.Printf("Failed to create v4 IPPool %v: %v \n", v4PoolName, err)
						return err
					}
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Println("create v6 ippool")
					v6PoolName, v6Pool = common.GenerateExampleIpv6poolObject(1)
					if frame.Info.SpiderSubnetEnabled {
						newv6SubnetName, newv6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 100)
						err = common.CreateSubnet(frame, newv6SubnetObject)
						if err != nil {
							GinkgoWriter.Printf("Failed to create v6 Subnet %v: %v \n", newv6SubnetName, err)
							return err
						}
						err = common.CreateIppoolInSpiderSubnet(ctx, frame, newv6SubnetName, v6Pool, 1)
					} else {
						err = common.CreateIppool(frame, v6Pool)
					}
					if err != nil {
						GinkgoWriter.Printf("Failed to create v6 IPPool %v: %v \n", v6PoolName, err)
						return err
					}
				}
				return nil
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeNil())

			DeferCleanup(func() {
				// Delete IPV4Pool and IPV6Pool
				if frame.Info.IpV4Enabled {
					GinkgoWriter.Printf("delete v4 ippool %v. \n", v4PoolName)
					Expect(common.DeleteIPPoolByName(frame, v4PoolName)).To(Succeed())
					if frame.Info.SpiderSubnetEnabled {
						Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
					}
				}
				if frame.Info.IpV6Enabled {
					GinkgoWriter.Printf("delete v6 ippool %v. \n", v6PoolName)
					Expect(common.DeleteIPPoolByName(frame, v6PoolName)).To(Succeed())
					if frame.Info.SpiderSubnetEnabled {
						Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
					}
				}
			})
		})

		It("use annotation `ipam.spidernet.io/ippools` with NIC name specified", func() {
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
				common.MultusNetworks:      fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan100),
			}
			Expect(podYaml).NotTo(BeNil())
			GinkgoWriter.Printf("succeeded to generate pod yaml: %+v. \n", podYaml)

			Expect(frame.CreatePod(podYaml)).To(Succeed())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
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
				ipv6Addr := net.ParseIP(v6Pool.Spec.IPs[0])
				command := fmt.Sprintf("ip -6 a show '%s' | grep '%s'", common.NIC2, ipv6Addr.String())
				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				errOut, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
				Expect(err).NotTo(HaveOccurred(), "failed to exec command %v, error is %v, %v \n", command, err, string(errOut))
			}

			GinkgoWriter.Printf("delete pod %v/%v. \n", nsName, podName)
			Expect(frame.DeletePod(podName, nsName)).To(Succeed())
		})

		It("use annotation `ipam.spidernet.io/ippools` with NIC name not specified", func() {
			// set pod annotation for nics
			podIppoolsAnno := types.AnnoPodIPPoolsValue{{}, {}}
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
				common.MultusNetworks:      fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan100),
			}
			Expect(podYaml).NotTo(BeNil())
			GinkgoWriter.Printf("succeeded to generate pod yaml: %+v. \n", podYaml)

			Expect(frame.CreatePod(podYaml)).To(Succeed())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
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
				ipv6Addr := net.ParseIP(v6Pool.Spec.IPs[0])
				command := fmt.Sprintf("ip -6 a show '%s' | grep '%s'", common.NIC2, ipv6Addr.String())
				ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
				defer cancel()
				errOut, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
				Expect(err).NotTo(HaveOccurred(), "failed to exec command %v, error is %v, %v \n", command, err, string(errOut))
			}

			GinkgoWriter.Printf("delete pod %v/%v. \n", nsName, podName)
			Expect(frame.DeletePod(podName, nsName)).To(Succeed())
		})

	})

	Context("wrong IPPools annotation usage", func() {
		It("It's invalid to specify one NIC corresponding IPPool in IPPools annotation with multiple NICs", Label("A00013"), func() {
			// set pod annotation for nics
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{common.SpiderPoolIPv4SubnetVlan100}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{common.SpiderPoolIPv6SubnetVlan100}
			}
			podIppoolsAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			podYaml.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPools: string(podIppoolsAnnoMarshal),
				common.MultusNetworks:      fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan100),
			}
			GinkgoWriter.Printf("succeeded to generate pod yaml with IPPools annotation: %+v. \n", podYaml)

			Expect(frame.CreatePod(podYaml)).To(Succeed())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
			defer cancel()
			GinkgoWriter.Printf("wait for one minute that pod %v/%v would not ready. \n", nsName, podName)
			_, err = frame.WaitPodStarted(podName, nsName, ctx)
			Expect(err).To(HaveOccurred())
		})

		It("It's invalid to specify same NIC name for IPPools annotation with multiple NICs", Label("A00014"), func() {
			// set pod annotation for nics
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				{
					NIC: common.NIC2,
				},
				{
					NIC: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				podIppoolsAnno[0].IPv4Pools = []string{common.SpiderPoolIPv4SubnetVlan100}
			}
			if frame.Info.IpV6Enabled {
				podIppoolsAnno[0].IPv6Pools = []string{common.SpiderPoolIPv6SubnetVlan100}
			}
			podIppoolsAnnoMarshal, err := json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			podYaml.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPools: string(podIppoolsAnnoMarshal),
				common.MultusNetworks:      fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan100),
			}
			GinkgoWriter.Printf("succeeded to generate pod yaml with same NIC name annotation: %+v. \n", podYaml)

			Expect(frame.CreatePod(podYaml)).To(Succeed())
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
			defer cancel()
			GinkgoWriter.Printf("wait for one minute that pod %v/%v would not ready. \n", nsName, podName)
			_, err = frame.WaitPodStarted(podName, nsName, ctx)
			Expect(err).To(HaveOccurred())
		})
	})

	It("Specify the default route NIC through Pod annotation: `ipam.spidernet.io/default-route-nic` ", Label("A00012"), func() {
		// make sure we have macvlan100 and macvlan200 net-attach-def resources
		_, err := frame.GetMultusInstance(common.MacvlanVlan100, common.MultusNs)
		if nil != err {
			if errors.IsNotFound(err) {
				Skip(fmt.Sprintf("no kubevirt multus CR '%s/%s' installed, ignore this suite", common.MultusNs, common.OvsVlan30))
			}
			Fail(err.Error())
		}
		_, err = frame.GetMultusInstance(common.MacvlanVlan200, common.MultusNs)
		if nil != err {
			if errors.IsNotFound(err) {
				Skip(fmt.Sprintf("no kubevirt multus CR '%s/%s' installed, ignore this suite", common.MultusNs, common.OvsVlan40))
			}
			Fail(err.Error())
		}

		// Generate example deploy yaml and create deploy
		deployName := "deploy-" + tools.RandomName()
		deployObj := common.GenerateExampleDeploymentYaml(deployName, nsName, 1)
		annotations := map[string]string{
			common.MultusDefaultNetwork:           fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan100),
			common.MultusNetworks:                 fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanVlan200),
			pkgconstant.AnnoDefaultRouteInterface: "net1",
		}
		if frame.Info.SpiderSubnetEnabled {
			subnetsAnno := []types.AnnoSubnetItem{
				{
					Interface: common.NIC1,
				},
				{
					Interface: common.NIC2,
				},
			}
			if frame.Info.IpV4Enabled {
				subnetsAnno[0].IPv4 = []string{common.SpiderPoolIPv4SubnetVlan100}
				subnetsAnno[1].IPv4 = []string{common.SpiderPoolIPv4SubnetVlan200}
			}
			if frame.Info.IpV6Enabled {
				subnetsAnno[0].IPv6 = []string{common.SpiderPoolIPv6SubnetVlan100}
				subnetsAnno[1].IPv6 = []string{common.SpiderPoolIPv6SubnetVlan200}
			}
			subnetsAnnoMarshal, err := json.Marshal(subnetsAnno)
			Expect(err).NotTo(HaveOccurred())
			annotations[pkgconstant.AnnoSpiderSubnets] = string(subnetsAnnoMarshal)
		}

		deployObj.Spec.Template.Annotations = annotations
		Expect(deployObj).NotTo(BeNil(), "failed to generate Deployment yaml")

		GinkgoWriter.Printf("Try to create deploy %v/%v \n", nsName, deployName)
		Expect(frame.CreateDeployment(deployObj)).To(Succeed())

		// Checking the pod run status should all be running.
		var podList *corev1.PodList
		Eventually(func() bool {
			podList, err = frame.GetPodListByLabel(deployObj.Spec.Template.Labels)
			if nil != err || len(podList.Items) == 0 {
				return false
			}
			return frame.CheckPodListRunning(podList)
		}, 2*common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

		Expect(podList.Items).To(HaveLen(1))
		podName = podList.Items[0].Name

		GinkgoWriter.Println("check whether the default route is same with the annotation value")
		net1DefaultGatewayV4 := "172.200.0.1"
		net1DefaultGatewayV6 := "fd00:172:200::1"
		if frame.Info.IpV4Enabled {
			command := fmt.Sprintf("ip r | grep 'default via %s dev net1'", net1DefaultGatewayV4)
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			out, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v , error is %v, %v \n", command, err, string(out))
		}
		if frame.Info.IpV6Enabled {
			command := fmt.Sprintf("ip -6 r | grep 'default via %s dev net1'", net1DefaultGatewayV6)
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			out, err := frame.ExecCommandInPod(podName, nsName, command, ctx)
			Expect(err).NotTo(HaveOccurred(), "failed to exec command %v , error is %v, %v \n", command, err, string(out))
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
