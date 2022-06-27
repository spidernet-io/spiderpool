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
	pkgconstant "github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("test annotation", Label("annotation"), func() {
	var nsName, podName string

	BeforeEach(func() {
		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
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
		var iPv4PoolObj, iPv6PoolObj *spiderpool.IPPool
		var ipv4vlan = new(spiderpool.Vlan)
		var ipv6vlan = new(spiderpool.Vlan)

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

			// // try to delete pod
			GinkgoWriter.Printf("try to delete pod %v/%v \n", nsName, podName)
			err = frame.DeletePod(podName, nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", nsName, podName)
		})
	})
	Context("annotation priority", func() {
		var v4PoolName, v6PoolName, nic, podIppoolAnnoStr, podIppoolsAnnoStr string
		var iPv4PoolObj, iPv6PoolObj *spiderpool.IPPool
		var v4PoolNameList, v6PoolNameList []string
		var defaultRouteBool bool
		BeforeEach(func() {
			nic = "eth0"
			defaultRouteBool = false
			// create ipv4pool
			if frame.Info.IpV4Enabled {
				// Generate v4PoolName and ipv4pool object
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(200)
				v4PoolNameList = append(v4PoolNameList, v4PoolName)
				GinkgoWriter.Printf("try to create ipv4pool: %v/%v \n", v4PoolName, iPv4PoolObj)
				err := common.CreateIppool(frame, iPv4PoolObj)
				Expect(err).NotTo(HaveOccurred(), "fail to create ipv4pool: %v \n", v4PoolName)
			}
			// create ipv6pool
			if frame.Info.IpV6Enabled {
				// Generate v6PoolName and ipv6pool object
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(200)
				v6PoolNameList = append(v6PoolNameList, v6PoolName)
				GinkgoWriter.Printf("try to create ipv6pool: %v/%v \n", v6PoolName, iPv6PoolObj)
				err := common.CreateIppool(frame, iPv6PoolObj)
				Expect(err).NotTo(HaveOccurred(), "fail to create ipv6pool: %v \n", v6PoolName)
			}
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
		It(`the "ippools" annotation has the higher priority over the "ippool" annotation`, Label("A00005"), func() {
			// get cluster default ipv4/ipv6 ippool
			clusterDefaultIPv4IPPoolList, clusterDefaultIPv6IPPoolList, err := common.GetClusterDefaultIppool(frame)
			Expect(err).NotTo(HaveOccurred())
			// ippool annotation
			podIppoolAnno := types.AnnoPodIPPoolValue{
				NIC:       &nic,
				IPv4Pools: clusterDefaultIPv4IPPoolList,
				IPv6Pools: clusterDefaultIPv6IPPoolList,
			}
			b, err := json.Marshal(podIppoolAnno)
			Expect(err).NotTo(HaveOccurred())
			podIppoolAnnoStr = string(b)
			// ippools annotation
			podIppoolsAnno := types.AnnoPodIPPoolsValue{
				types.AnnoIPPoolItem{
					NIC:          nic,
					IPv4Pools:    v4PoolNameList,
					IPv6Pools:    v6PoolNameList,
					DefaultRoute: defaultRouteBool,
				},
			}
			b, err = json.Marshal(podIppoolsAnno)
			Expect(err).NotTo(HaveOccurred())
			podIppoolsAnnoStr = string(b)
			// Generate Pod Yaml
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			Expect(podYaml).NotTo(BeNil())
			podYaml.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPool:  podIppoolAnnoStr,
				pkgconstant.AnnoPodIPPools: podIppoolsAnnoStr,
			}
			GinkgoWriter.Printf("podYaml: %v \n", podYaml)
			pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, time.Second*30)
			GinkgoWriter.Printf("pod %v/%v: podIPv4: %v, podIPv6: %v \n", nsName, podName, podIPv4, podIPv6)

			// Check pod ip in v4PoolName、v6PoolName
			v := &corev1.PodList{
				Items: []corev1.Pod{*pod},
			}
			ok, _, _, e := common.CheckPodIpRecordInIppool(frame, v4PoolNameList, v6PoolNameList, v)
			Expect(e).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			// try to delete pod
			GinkgoWriter.Printf("try to delete pod %v/%v \n", nsName, podName)
			err = frame.DeletePod(podName, nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", nsName, podName)
		})
	})

	Context("ippool priority", func() {
		var deployName, v4PoolName1, v4PoolName2, v6PoolName1, v6PoolName2, nic, podAnnoStr string
		var v4PoolObj1, v4PoolObj2, v6PoolObj1, v6PoolObj2 *spiderpool.IPPool

		BeforeEach(func() {
			// label namespace
			ns, e1 := frame.GetNamespace(nsName)
			Expect(e1).NotTo(HaveOccurred())
			Expect(ns).NotTo(BeNil())

			ns.Labels = map[string]string{
				"namespace": nsName,
			}
			Expect(frame.UpdateResource(ns)).To(Succeed())

			nic = "eth0"

			// create ippool
			if frame.Info.IpV4Enabled {
				// create ippool v4PoolName1
				v4PoolName1, v4PoolObj1 = common.GenerateExampleIpv4poolObject(3)
				createIPPool(v4PoolObj1)

				// create ippool v4PoolName2
				v4PoolName2, v4PoolObj2 = common.GenerateExampleIpv4poolObject(3)
				v4PoolObj2.Spec.NodeSelector = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"namespace": nsName,
					},
				}
				createIPPool(v4PoolObj2)
			}
			if frame.Info.IpV6Enabled {
				// create ippool v6PoolName1
				v6PoolName1, v6PoolObj1 = common.GenerateExampleIpv6poolObject(3)
				createIPPool(v6PoolObj1)

				// create ippool v6PoolName2
				v6PoolName2, v6PoolObj2 = common.GenerateExampleIpv6poolObject(3)
				v6PoolObj2.Spec.NodeSelector = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"namespace": nsName,
					},
				}
				createIPPool(v6PoolObj2)
			}

			deployName = "deploy" + tools.RandomName()

			// pod annotations
			podAnno := types.AnnoPodIPPoolValue{
				NIC: &nic,
			}
			if frame.Info.IpV4Enabled {
				podAnno.IPv4Pools = []string{v4PoolName1}
			}
			if frame.Info.IpV6Enabled {
				podAnno.IPv6Pools = []string{v6PoolName1}
			}
			b, e2 := json.Marshal(podAnno)
			Expect(e2).NotTo(HaveOccurred())
			podAnnoStr = string(b)

			DeferCleanup(func() {
				// delete ippool
				if frame.Info.IpV4Enabled {
					deleteIPPoolUntilFinish(v4PoolName1)
					deleteIPPoolUntilFinish(v4PoolName2)
				}
				if frame.Info.IpV6Enabled {
					deleteIPPoolUntilFinish(v6PoolName1)
					deleteIPPoolUntilFinish(v6PoolName2)
				}
			})
		})

		It("the pod annotation has the highest priority over namespace and global default ippool", Label("A00004"), func() {
			// generate deployment yaml
			GinkgoWriter.Println("generate deploy yaml")
			deployYaml := common.GenerateExampleDeploymentYaml(deployName, nsName, int32(3))

			deployYaml.Spec.Template.Annotations = map[string]string{pkgconstant.AnnoPodIPPool: podAnnoStr}

			// create deployment until ready
			deploy, e1 := frame.CreateDeploymentUntilReady(deployYaml, time.Minute)
			Expect(e1).NotTo(HaveOccurred())

			// get podList
			GinkgoWriter.Println("get podList")
			podList, e2 := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
			Expect(e2).NotTo(HaveOccurred())
			GinkgoWriter.Printf("podList Num:%v\n", len(podList.Items))

			// check podIP record in IPPool
			GinkgoWriter.Println("check podIP record in ippool --- pod level")
			ok3, _, _, e3 := common.CheckPodIpRecordInIppool(frame, []string{v4PoolName1}, []string{v6PoolName1}, podList)
			Expect(e3).NotTo(HaveOccurred())
			Expect(ok3).To(BeTrue())

			// scale deployment and check ip recorded in ippool --- pod and namespace level
			scaleDeployCheckIpRecord(deploy, nsName, 6, []string{v4PoolName1, v4PoolName2}, []string{v6PoolName1, v6PoolName2}, time.Minute)

			// scale deployment and check ip recorded in ippool --- pod、namespace and cluster level
			v4poolNames := append(ClusterDefaultV4IpoolList, v4PoolName1, v4PoolName2)
			v6poolNames := append(ClusterDefaultV6IpoolList, v6PoolName1, v6PoolName2)
			scaleDeployCheckIpRecord(deploy, nsName, 9, v4poolNames, v6poolNames, time.Minute)

			// delete deployment
			GinkgoWriter.Printf("delete deployment %v/%v\n", nsName, deployName)
			Expect(frame.DeleteDeploymentUntilFinish(deployName, nsName, time.Minute)).To(Succeed())
		})
	})
})

func createIPPool(IPPoolObj *spiderpool.IPPool) {
	GinkgoWriter.Printf("create ippool %v\n", IPPoolObj.Name)
	Expect(common.CreateIppool(frame, IPPoolObj)).To(Succeed())
	GinkgoWriter.Printf("create ippool %v succceed\n", IPPoolObj.Name)
}

func deleteIPPoolUntilFinish(poolName string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	GinkgoWriter.Printf("delete ippool %v\n", poolName)
	Expect(common.DeleteIPPoolUntilFinish(frame, poolName, ctx)).To(Succeed())
}

func scaleDeployCheckIpRecord(deploy *appsv1.Deployment, namespace string, scaleNum int, v4PoolNames, v6PoolNames []string, timeOut time.Duration) {
	var e error
	// scale deployment
	GinkgoWriter.Println("scale deployment")
	_, e = frame.ScaleDeployment(deploy, int32(scaleNum))
	Expect(e).NotTo(HaveOccurred())

	// wait deployment ready
	GinkgoWriter.Printf("wait deployment ready %v/%v\n", namespace, deploy.Name)
	ctx, cancel := context.WithTimeout(context.Background(), timeOut)
	defer cancel()
	deployment, e := frame.WaitDeploymentReady(deploy.Name, namespace, ctx)
	Expect(deployment).NotTo(BeNil())
	Expect(e).NotTo(HaveOccurred())

	// get pod list
	GinkgoWriter.Println("get podList")
	podList, e := frame.GetPodListByLabel(deploy.Spec.Selector.MatchLabels)
	Expect(e).NotTo(HaveOccurred())
	GinkgoWriter.Printf("podList Num:%v\n", len(podList.Items))

	// check podIP record in IPPool
	GinkgoWriter.Println("check podIP record in ippool")
	ok, _, _, e := common.CheckPodIpRecordInIppool(frame, v4PoolNames, v6PoolNames, podList)
	Expect(e).NotTo(HaveOccurred())
	Expect(ok).To(BeTrue())
}
