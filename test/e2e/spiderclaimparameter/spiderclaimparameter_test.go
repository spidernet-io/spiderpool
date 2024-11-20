// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package spiderclaimparameter_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("Spiderclaimparameter", Label("SpiderClaimParameter"), func() {
	Context("Test Spiderclaimparameter", func() {
		var namespace, multusNadName, spiderClaimName string

		BeforeEach(func() {
			// generate some test data
			namespace = "ns-" + common.GenerateString(10, true)
			multusNadName = "test-multus-" + common.GenerateString(10, true)
			spiderClaimName = "spc-" + common.GenerateString(10, true)

			// create namespace and ippool
			err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
			Expect(err).NotTo(HaveOccurred())

			DeferCleanup(func() {
				GinkgoWriter.Printf("delete spiderMultusConfig %v/%v. \n", namespace, multusNadName)
				Expect(frame.DeleteSpiderMultusInstance(namespace, multusNadName)).NotTo(HaveOccurred())

				GinkgoWriter.Printf("delete namespace %v. \n", namespace)
				Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())

				//Expect(
				//	common.DeleteSpiderClaimParameter(frame, spiderClaimName, namespace),
				//).NotTo(HaveOccurred())
			})
		})

		It(("test create spiderclaimparameter"), Label("Y00001"), func() {
			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
						VlanID: ptr.To(int32(100)),
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						PodDefaultRouteNIC: &common.NIC2,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			defer func() {
				Expect(frame.DeleteSpiderMultusInstance(namespace, multusNadName)).NotTo(HaveOccurred())
			}()

			Expect(common.CreateSpiderClaimParameter(frame, &spiderpoolv2beta1.SpiderClaimParameter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderClaimName,
					Namespace: namespace,
					// kind k8s v1.29.0 -> use containerd v1.7.1 -> use cdi version(v0.5.4)
					// v0.5.4 don't support CDISpec version 0.6.0, so update the cdi version
					// by the annotation
					Annotations: map[string]string{
						constant.AnnoDraCdiVersion: "0.5.0",
					},
				},
				Spec: spiderpoolv2beta1.ClaimParameterSpec{
					// RdmaAcc: true,
					SecondaryNics: []spiderpoolv2beta1.MultusConfig{{
						MultusName: multusNadName,
						Namespace:  namespace,
					},
					}},
			})).NotTo(HaveOccurred())
		})

		It(("test create spiderclaimparameter for empty staticNics"), Label("Y00002"), func() {
			// Define multus cni NetworkAttachmentDefinition and create
			nad := &spiderpoolv2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      multusNadName,
					Namespace: namespace,
				},
				Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
						VlanID: ptr.To(int32(100)),
					},
					CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
						PodDefaultRouteNIC: &common.NIC2,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())

			defer func() {
				Expect(frame.DeleteSpiderMultusInstance(namespace, multusNadName)).NotTo(HaveOccurred())
			}()

			err := common.CreateSpiderClaimParameter(frame, &spiderpoolv2beta1.SpiderClaimParameter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderClaimName,
					Namespace: namespace,
					// kind k8s v1.29.0 -> use containerd v1.7.1 -> use cdi version(v0.5.4)
					// v0.5.4 don't support CDISpec version 0.6.0, so update the cdi version
					// by the annotation
					Annotations: map[string]string{
						constant.AnnoDraCdiVersion: "0.5.0",
					},
				},
				Spec: spiderpoolv2beta1.ClaimParameterSpec{
					// RdmaAcc: true,
					SecondaryNics: []spiderpoolv2beta1.MultusConfig{{
						MultusName: multusNadName,
						Namespace:  namespace,
					},
					}},
			})

			Expect(err).To(HaveOccurred(), "expect err: %v", err)
		})

		It(("test create spiderclaimparameter for no found spiderMultusConfig"), func() {
			err := common.CreateSpiderClaimParameter(frame, &spiderpoolv2beta1.SpiderClaimParameter{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderClaimName,
					Namespace: namespace,
					// kind k8s v1.29.0 -> use containerd v1.7.1 -> use cdi version(v0.5.4)
					// v0.5.4 don't support CDISpec version 0.6.0, so update the cdi version
					// by the annotation
					Annotations: map[string]string{
						constant.AnnoDraCdiVersion: "0.5.0",
					},
				},
				Spec: spiderpoolv2beta1.ClaimParameterSpec{
					// RdmaAcc: true,
					SecondaryNics: []spiderpoolv2beta1.MultusConfig{{
						MultusName: multusNadName,
						Namespace:  namespace,
					},
					}},
			})

			Expect(err).To(HaveOccurred(), "expect err: %v", err)
		})
	})
})
