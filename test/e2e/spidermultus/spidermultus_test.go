// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package spidermultus_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("test spidermultus", Label("spiderMultus", "overlay"), func() {

	Context("Creation, update, deletion of spider multus", func() {
		var namespace, mode, multusNadName, podCidrType string

		BeforeEach(func() {
			multusNadName = "test-multus-" + common.GenerateString(10, true)
			mode = "disabled"
			podCidrType = "cluster"
			namespace = "ns-" + common.GenerateString(10, true)

			// create namespace
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: "macvlan",
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						Mode:        &mode, //	mode = "disabled"
						PodCIDRType: podCidrType,
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
			spiderMultusConfig, err := frame.GetSpiderMultusInstance(namespace, multusNadName)
			Expect(err).NotTo(HaveOccurred())
			Expect(spiderMultusConfig).NotTo(BeNil())
			GinkgoWriter.Printf("spiderMultusConfig %+v \n", spiderMultusConfig)

			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(multusNadName, namespace)
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
			err = frame.DeleteMultusInstance(multusNadName, namespace)
			Expect(err).NotTo(HaveOccurred())
			multusConfig, err := frame.GetMultusInstance(multusNadName, namespace)
			Expect(api_errors.IsNotFound(err)).To(BeTrue())
			Expect(multusConfig).To(BeNil())

			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(multusNadName, namespace)
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
			err = frame.DeleteSpiderMultusInstance(namespace, multusNadName)
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				_, err := frame.GetMultusInstance(multusNadName, namespace)
				return api_errors.IsNotFound(err)
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())
		})
	})
})
