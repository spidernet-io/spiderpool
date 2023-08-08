// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package spidermultus_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test spidermultus", Label("spiderMultus", "overlay"), func() {

	Context("Creation, update, deletion of spider multus", func() {
		var namespace, mode, spiderMultusNadName, podCidrType string

		BeforeEach(func() {
			spiderMultusNadName = "test-multus-" + common.GenerateString(10, true)
			mode = "disabled"
			podCidrType = "cluster"
			namespace = "ns-" + common.GenerateString(10, true)

			// create namespace
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderMultusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: "macvlan",
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						Mode:        &mode, //	mode = "disabled"
						PodCIDRType: &podCidrType,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			// Clean test env
			DeferCleanup(func() {
				err := frame.DeleteNamespace(namespace)
				Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace %v")
			})
		})

		It(`Delete multus nad and spidermultus, the deletion of the former will be automatically restored, 
		    and the deletion of the latter will clean up all resources synchronously`, Label("M00001", "M00002", "M00004"), func() {
			spiderMultusConfig, err := frame.GetSpiderMultusInstance(namespace, spiderMultusNadName)
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderMultusConfig).NotTo(BeNil())
			GinkgoWriter.Printf("spiderMultusConfig %+v \n", spiderMultusConfig)

			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(spiderMultusNadName, namespace)
				GinkgoWriter.Printf("Auto-generated multus configuration %+v \n", multusConfig)
				if api_errors.IsNotFound(err) {
					return false
				}
				// The automatically generated multus configuration should be associated with spidermultus
				if multusConfig.ObjectMeta.OwnerReferences[0].Kind != constant.KindSpiderMultusConfig {
					return false
				}
				return true
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())

			// Delete the multus configuration created automatically,
			// and it will be restored automatically after a period of time.
			err = frame.DeleteMultusInstance(spiderMultusNadName, namespace)
			Expect(err).NotTo(HaveOccurred())
			multusConfig, err := frame.GetMultusInstance(spiderMultusNadName, namespace)
			Expect(api_errors.IsNotFound(err)).To(BeTrue())
			Expect(multusConfig).To(BeNil())

			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(spiderMultusNadName, namespace)
				GinkgoWriter.Printf("multus configuration  %+v automatically restored after deletion \n", multusConfig)
				if api_errors.IsNotFound(err) {
					return false
				}
				if multusConfig.ObjectMeta.OwnerReferences[0].Kind != constant.KindSpiderMultusConfig {
					return false
				}
				return true
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())

			// After spidermultus is deleted, multus will be deleted synchronously.
			err = frame.DeleteSpiderMultusInstance(namespace, spiderMultusNadName)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				_, err := frame.GetMultusInstance(spiderMultusNadName, namespace)
				return api_errors.IsNotFound(err)
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())
		})
	})

	Context("Change multus attributes via spidermultus annotation", func() {
		var namespace, spiderMultusNadName, mode string
		var smc *spiderpoolv2beta1.SpiderMultusConfig

		BeforeEach(func() {
			spiderMultusNadName = "test-multus-" + common.GenerateString(10, true)
			namespace = "ns-" + common.GenerateString(10, true)
			mode = "disabled"

			// create namespace
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			// Define spidermultus cr and create
			smc = &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderMultusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: "macvlan",
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						Mode: &mode,
					},
				},
			}
			GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)

			// Clean test env
			// DeferCleanup(func() {
			// 	err := frame.DeleteNamespace(namespace)
			// 	Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace %v")
			// })
		})

		It("Customize net-attach-conf name via annotation multus.spidernet.io/cr-name", Label("M00005"), func() {
			multusNadName := "test-custom-multus-" + common.GenerateString(10, true)
			smc.ObjectMeta.Annotations = map[string]string{constant.AnnoNetAttachConfName: multusNadName}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", smc)

			Expect(frame.CreateSpiderMultusInstance(smc)).NotTo(HaveOccurred())

			spiderMultusConfig, err := frame.GetSpiderMultusInstance(namespace, spiderMultusNadName)
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderMultusConfig).NotTo(BeNil())
			GinkgoWriter.Printf("spiderMultusConfig %+v \n", spiderMultusConfig)

			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(multusNadName, namespace)
				GinkgoWriter.Printf("Auto-generated multus configuration %+v \n", multusConfig)
				if api_errors.IsNotFound(err) {
					return false
				}
				if multusConfig.ObjectMeta.OwnerReferences[0].Kind != constant.KindSpiderMultusConfig {
					return false
				}
				return true
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())
		})

		It("annotating custom names that are too long or empty should fail", Label("M00009"), func() {
			longCustomizedName := common.GenerateString(k8svalidation.DNS1123SubdomainMaxLength+1, true)
			smc.ObjectMeta.Annotations = map[string]string{constant.AnnoNetAttachConfName: longCustomizedName}
			GinkgoWriter.Printf("spidermultus cr with annotations: '%+v' \n", smc)
			Expect(frame.CreateSpiderMultusInstance(smc)).To(HaveOccurred())

			emptyCustomizedName := ""
			smc.ObjectMeta.Annotations = map[string]string{constant.AnnoNetAttachConfName: emptyCustomizedName}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", smc)
			Expect(frame.CreateSpiderMultusInstance(smc)).To(HaveOccurred())
		})

		It("Change net-attach-conf version via annotation multus.spidernet.io/cni-version", Label("M00006"), func() {
			cniVersion := "0.4.0"
			smc.ObjectMeta.Annotations = map[string]string{constant.AnnoMultusConfigCNIVersion: cniVersion}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", smc)
			Expect(frame.CreateSpiderMultusInstance(smc)).NotTo(HaveOccurred())

			spiderMultusConfig, err := frame.GetSpiderMultusInstance(namespace, spiderMultusNadName)
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderMultusConfig).NotTo(BeNil())
			GinkgoWriter.Printf("spiderMultusConfig %+v \n", spiderMultusConfig)

			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(spiderMultusNadName, namespace)
				GinkgoWriter.Printf("Auto-generated multus configuration %+v \n", multusConfig)
				if api_errors.IsNotFound(err) {
					return false
				}
				// The cni version should match.
				if multusConfig.ObjectMeta.Annotations[constant.AnnoMultusConfigCNIVersion] != cniVersion {
					return false
				}
				return true
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())
		})

		It("fail to customize unsupported CNI version", Label("M00006"), func() {
			mismatchCNIVersion := "x.y.z"
			smc.ObjectMeta.Annotations = map[string]string{constant.AnnoMultusConfigCNIVersion: mismatchCNIVersion}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", smc)
			// Mismatched versions, when doing a build, the error should occur here?
			Expect(frame.CreateSpiderMultusInstance(smc)).To(HaveOccurred())
		})
	})
})
