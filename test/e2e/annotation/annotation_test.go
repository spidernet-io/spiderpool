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
	corev1 "k8s.io/api/core/v1"
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
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject()
				iPv4PoolObj.Spec.Vlan = ipv4vlan
				GinkgoWriter.Printf("try to create ipv4pool: %v/%v \n", v4PoolName, iPv4PoolObj)
				err := common.CreateIppool(frame, iPv4PoolObj)
				Expect(err).NotTo(HaveOccurred(), "fail to create ipv4pool %v \n", v4PoolName)
			}
			// create ipv6pool
			if frame.Info.IpV6Enabled {
				// Generate v6PoolName and ipv6pool object
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject()
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
})
