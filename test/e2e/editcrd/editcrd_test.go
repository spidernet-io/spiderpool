// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package editcrd_test

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
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("test editcrd", Label("editcrd"), func() {
	var nsName, podName string

	BeforeEach(func() {
		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)
		// init pod name
		podName = "pod" + tools.RandomName()
		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})
	Context("about edit crd", func() {
		var v4PoolName, v6PoolName, nic, podAnnoStr, podName2 string
		var iPv4PoolObj, iPv6PoolObj *spiderpool.IPPool
		var disable = new(bool)

		BeforeEach(func() {
			nic = "eth0"
			*disable = true
			podName2 = "pod" + tools.RandomName()
			if frame.Info.IpV4Enabled {
				v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject()
				GinkgoWriter.Printf("try to create ipv4pool: %v/%v \n", v4PoolName, iPv4PoolObj)
				err := common.CreateIppool(frame, iPv4PoolObj)
				Expect(err).NotTo(HaveOccurred(), "fail to create ipv4pool: %v \n", v4PoolName)
			}
			if frame.Info.IpV6Enabled {
				v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject()
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
		It(`D00005: a "true" value of ippool.Spec.disabled should fobide IP allocation, but still allow ip deallocation \n
		    D00004: it fails to delete an ippool whose IP is not deallocated at all`, Label("D00005", "D00004"), func() {
			// ippool annotation
			podIppoolAnno := types.AnnoPodIPPoolValue{
				NIC:       &nic,
				IPv4Pools: []string{v4PoolName},
				IPv6Pools: []string{v6PoolName},
			}
			b, err := json.Marshal(podIppoolAnno)
			Expect(err).NotTo(HaveOccurred())
			podAnnoStr = string(b)
			// Generate Pod Yaml
			podYaml := common.GenerateExamplePodYaml(podName, nsName)
			Expect(podYaml).NotTo(BeNil())
			podYaml.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPool: podAnnoStr,
			}
			pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, time.Second*30)
			GinkgoWriter.Printf("pod %v/%v: podIPv4: %v, podIPv6: %v \n", nsName, podName, podIPv4, podIPv6)

			// Check pod ip in v4PoolName、v6PoolName
			v := &corev1.PodList{
				Items: []corev1.Pod{*pod},
			}
			ok, _, _, e := common.CheckPodIpRecordInIppool(frame, []string{v4PoolName}, []string{v6PoolName}, v)
			Expect(e).NotTo(HaveOccurred())
			Expect(ok).To(BeTrue())

			// TODO(tao.yang), D00004: it fails to delete an ippool whose IP is not deallocated at all
			// so, deleting ippool should fail

			// set iPv4/iPv6 PoolObj.Spec.Disable to true
			if frame.Info.IpV4Enabled {
				iPv4PoolObj = common.GetIppoolByName(frame, v4PoolName)
				iPv4PoolObj.Spec.Disable = disable
				err = common.UpdateIppool(frame, iPv4PoolObj)
				Expect(err).NotTo(HaveOccurred())
			}
			if frame.Info.IpV6Enabled {
				iPv6PoolObj = common.GetIppoolByName(frame, v6PoolName)
				iPv6PoolObj.Spec.Disable = disable
				err = common.UpdateIppool(frame, iPv6PoolObj)
				Expect(err).NotTo(HaveOccurred())
			}

			// Create a new pod again, name is podName2
			podYaml = common.GenerateExamplePodYaml(podName2, nsName)
			Expect(podYaml).NotTo(BeNil())
			podYaml.Annotations = map[string]string{
				pkgconstant.AnnoPodIPPool: podAnnoStr,
			}
			// TODO(tao.yang), after set iPv4/iPv6 PoolObj.Spec.Disable to true，failed to run pod
			// err = frame.CreatePod(podYaml)
			// Expect(err).NotTo(HaveOccurred())
			// ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			// defer cancel()
			// err = frame.WaitExceptEventOccurred(ctx, common.PodEventKind, podName, nsName, common.CNIFailedToSetUpNetwork)
			// Expect(err).NotTo(HaveOccurred(), "fail to get event %v/%v = %v\n", nsName, podName, common.CNIFailedToSetUpNetwork)

			// try to delete pod，name is podName
			GinkgoWriter.Printf("try to delete pod %v/%v \n", nsName, podName)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
			defer cancel()
			opt := &client.DeleteOptions{
				GracePeriodSeconds: pointer.Int64Ptr(0),
			}
			Expect(frame.DeletePodUntilFinish(podName, nsName, ctx, opt)).To(Succeed())
			GinkgoWriter.Printf("delete pod %v/%v successfully\n", nsName, podName)

			// check if the pod ip in ippool reclaimed normally
			GinkgoWriter.Println("check ip is release successfully")
			Expect(common.WaitIPReclaimedFinish(frame, []string{v4PoolName}, []string{v6PoolName}, v, time.Minute)).To(Succeed())
		})
	})
})
