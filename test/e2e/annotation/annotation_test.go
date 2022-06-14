// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package annotation_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	anno "github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test annotation", Label("annotation"), func() {
	var nsName, podName, ipv4poolName1, ipv6poolName1, ipv4poolName2, ipv6poolName2 string
	var IPVersion spiderpool.IPVersion = "IPv6"
	BeforeEach(func() {
		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("try to create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "fail to create namespace %v", nsName)

		ipv4poolName1 = "v4pool" + tools.RandomName()
		ipv6poolName1 = "v6pool" + tools.RandomName()
		GinkgoWriter.Printf("try to create ipv4poolName1 %v,ipv6poolName1 %v \n", ipv4poolName1, ipv6poolName1)
		poolYaml := common.GenerateExampleIppoolYaml(ipv4poolName1)
		poolYaml.Spec.IPs = []string{"172.21.40.2-172.21.21.254"}
		poolYaml.Spec.Subnet = "172.21.0.0/16"
		err = common.CreateIppool(frame, poolYaml)
		Expect(err).NotTo(HaveOccurred(), "fail to create ippool %v", ipv4poolName1)
		poolYaml = common.GenerateExampleIppoolYaml(ipv6poolName1)
		poolYaml.Spec.IPs = []string{"fc00:f855:ccd:e793:f::2-fc00:f855:ccd:e793:f::fe"}
		poolYaml.Spec.Subnet = "fc00:f855:ccd:e793::/64"
		poolYaml.Spec.IPVersion = &IPVersion
		err = common.CreateIppool(frame, poolYaml)
		Expect(err).NotTo(HaveOccurred(), "fail to create ippool %v", ipv6poolName1)

		ipv4poolName2 = "v4pool" + tools.RandomName()
		ipv6poolName2 = "v6pool" + tools.RandomName()
		GinkgoWriter.Printf("try to create ipv4poolName2 %v,ipv6poolName2 %v \n", ipv4poolName2, ipv6poolName2)
		poolYaml = common.GenerateExampleIppoolYaml(ipv4poolName2)
		poolYaml.Spec.IPs = []string{"172.22.40.2-172.22.21.254"}
		poolYaml.Spec.Subnet = "172.22.0.0/16"
		err = common.CreateIppool(frame, poolYaml)
		Expect(err).NotTo(HaveOccurred(), "fail to create ippool %v", ipv4poolName2)
		poolYaml = common.GenerateExampleIppoolYaml(ipv6poolName2)
		poolYaml.Spec.IPs = []string{"fc00:f856:ccd:e793:f::2-fc00:f856:ccd:e793:f::fe"}
		poolYaml.Spec.Subnet = "fc00:f856:ccd:e793::/64"
		poolYaml.Spec.IPVersion = &IPVersion
		err = common.CreateIppool(frame, poolYaml)
		Expect(err).NotTo(HaveOccurred(), "fail to create ippool %v", ipv6poolName2)

		// init test name
		podName = "pod" + tools.RandomName()
		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("try to delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)

			GinkgoWriter.Printf("try to delete ipv4pool1 and ipv6pool1 %v %v \n", ipv4poolName1, ipv6poolName1)
			err = common.DeleteIppool(frame, ipv4poolName1)
			Expect(err).NotTo(HaveOccurred(), "fail to delete ipv4ippool %v", ipv4poolName1)
			err = common.DeleteIppool(frame, ipv6poolName1)
			Expect(err).NotTo(HaveOccurred(), "fail to delete ipv6ippool %v", ipv6poolName1)

			GinkgoWriter.Printf("try to delete ipv4pool2 and ipv6pool2 %v %v \n", ipv4poolName2, ipv6poolName2)
			err = common.DeleteIppool(frame, ipv4poolName2)
			Expect(err).NotTo(HaveOccurred(), "fail to delete ipv4ippool %v", ipv4poolName2)
			err = common.DeleteIppool(frame, ipv6poolName2)
			Expect(err).NotTo(HaveOccurred(), "fail to delete ipv6ippool %v", ipv6poolName2)
		})
	})

	It("the 'ippools' annotation has the higher priority over the 'ippool' annotation", Label("A00005"), func() {
		podYaml := common.GenerateExamplePodYaml(podName, nsName)
		Expect(podYaml).NotTo(BeNil())
		podYaml.Annotations = map[string]string{
			anno.AnnoPodIPPool:  fmt.Sprintf(`{"interface":"eth0","ipv4pools":["%v"],"ipv6pools":["%v"]}`, ipv4poolName1, ipv6poolName1),
			anno.AnnoPodIPPools: fmt.Sprintf(`[{"interface":"eth0","ipv4pools":["%v"],"ipv6pools":["%v"],"defaultRoute":true}]`, "default-v4-ippool", "default-v6-ippool"),
		}
		GinkgoWriter.Printf("podYaml: %v \n", podYaml)
		pod, podIPv4, podIPv6 := common.CreatePodUntilReady(frame, podYaml, podName, nsName, time.Second*30)
		GinkgoWriter.Printf("podIPv4: %v; podIPv6: %v\n", podIPv4, podIPv6)
		Expect(pod.Annotations[anno.AnnoPodIPPool]).To(Equal(podYaml.Annotations[anno.AnnoPodIPPool]))
		Expect(pod.Annotations[anno.AnnoPodIPPools]).To(Equal(podYaml.Annotations[anno.AnnoPodIPPools]))

		v := &corev1.PodList{
			Items: []corev1.Pod{*pod},
		}
		ok, e := common.CheckPodIpRecordInIppool(frame, []string{"default-v4-ippool"}, []string{"default-v6-ippool"}, v)
		if e != nil || !ok {
			Fail(fmt.Sprintf("failed to CheckPodIpRecordInIppool, reason=%v", e))
		}

		// delete pod
		GinkgoWriter.Printf("try to delete pod %v/%v \n", nsName, podName)
		err := frame.DeletePod(podName, nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", nsName, podName)
	})
})
