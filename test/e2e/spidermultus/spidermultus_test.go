// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package spidermultus_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types"
	v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	netutils "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/utils"

	"github.com/onsi/ginkgo/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test spidermultus", Label("SpiderMultusConfig"), func() {
	var namespace string

	BeforeEach(func() {
		// create namespace
		namespace = "ns-" + common.GenerateString(10, true)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "Failed to delete namespace %v")
		})
	})

	Context("Creation, update, deletion of spider multus", func() {
		var mode, spiderMultusNadName, podCidrType string

		BeforeEach(func() {
			spiderMultusNadName = "test-multus-" + common.GenerateString(10, true)
			mode = "disabled"
			podCidrType = "cluster"

			// Define multus cni NetworkAttachmentDefinition and create
			nad := &v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderMultusNadName,
					Namespace: namespace,
				},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &v2beta1.CoordinatorSpec{
						Mode:        &mode, //	mode = "disabled"
						PodCIDRType: &podCidrType,
					},
				},
			}
			Expect(frame.CreateSpiderMultusInstance(nad)).NotTo(HaveOccurred())
		})

		It(`Delete multus nad and spidermultus, the deletion of the former will be automatically restored, 
		    and the deletion of the latter will clean up all resources synchronously`, Label("M00001", "M00007", "M00008"), func() {
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
		var spiderMultusNadName, mode string
		var smc *v2beta1.SpiderMultusConfig

		BeforeEach(func() {
			spiderMultusNadName = "test-multus-" + common.GenerateString(10, true)
			mode = "disabled"

			// Define spidermultus cr and create
			smc = &v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      spiderMultusNadName,
					Namespace: namespace,
				},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType: ptr.To(constant.MacvlanCNI),
					MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
						Master: []string{common.NIC1},
					},
					CoordinatorConfig: &v2beta1.CoordinatorSpec{
						Mode: &mode,
					},
				},
			}
			GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		})

		It("Customize net-attach-conf name via annotation multus.spidernet.io/cr-name", Label("M00010"), func() {
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

		It("webhook validation: New and existing SpiderMultusConfig in the same namespace with the same customMultusName will not be created due to a conflict.", Label("M00011"), func() {
			// Create SpiderMultusConfig and customize the net-attach-def name by annotating multus.spidernet.io/cr-name
			testSmc := smc.DeepCopy()
			testSmc.Name = "test-smc-1-" + common.GenerateString(10, true)
			sameCustomMultusName := "test-custom-multus-" + common.GenerateString(10, true)
			testSmc.Annotations = map[string]string{constant.AnnoNetAttachConfName: sameCustomMultusName}
			GinkgoWriter.Printf("spidermultus cr with annotations: '%+v' \n", testSmc.Annotations)
			Expect(frame.CreateSpiderMultusInstance(testSmc)).NotTo(HaveOccurred())

			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(sameCustomMultusName, testSmc.Namespace)
				GinkgoWriter.Printf("Auto-generated multus configuration %+v \n", multusConfig)
				return !api_errors.IsNotFound(err)
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())

			// Create another SpiderMultusConfig with the same custom net-attach-def name
			newSmc := smc.DeepCopy()
			newSmc.Name = "test-another-smc-1-" + common.GenerateString(10, true)
			newSmc.Annotations = map[string]string{constant.AnnoNetAttachConfName: sameCustomMultusName}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", newSmc.Annotations)
			err := frame.CreateSpiderMultusInstance(newSmc)
			errorMsg := fmt.Sprintf("the net-attach-def %s/%s already exists and is managed by SpiderMultusConfig %s/%s.",
				newSmc.Namespace, sameCustomMultusName, testSmc.Namespace, testSmc.Name)
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))
		})

		It("webhook validation: the custom net-attach-def name of SpiderMultusConfig conflicts with the existing SpiderMultusConfig name, and cannot be created.", Label("M00011"), func() {
			// Create SpiderMultusConfig in advance
			testSmc := smc.DeepCopy()
			testSmc.Name = "test-smc-2-" + common.GenerateString(10, true)
			Expect(frame.CreateSpiderMultusInstance(testSmc)).NotTo(HaveOccurred())
			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(testSmc.Name, testSmc.Namespace)
				GinkgoWriter.Printf("Auto-generated multus configuration %+v \n", multusConfig)
				return !api_errors.IsNotFound(err)
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())

			// New SpiderMultusConfig's custom net-attach-def name conflicts with existing SpiderMultusConfig's name
			newSmc := smc.DeepCopy()
			newSmc.Name = "test-another-smc-2-" + common.GenerateString(10, true)
			newSmc.Annotations = map[string]string{constant.AnnoNetAttachConfName: testSmc.Name}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", newSmc.Annotations)
			err := frame.CreateSpiderMultusInstance(newSmc)
			GinkgoWriter.Printf("should fail to create, the error is: %v", err.Error())
			errorMsg := fmt.Sprintf("the net-attach-def %s/%s already exists and is managed by SpiderMultusConfig %s/%s.",
				newSmc.Namespace, testSmc.Name, testSmc.Namespace, testSmc.Name)
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))
		})

		It("webhook validation: the name of SpiderMultusConfig conflicts with the custom net-attach-def name of an existing SpiderMultusConfig, and cannot be created.", Label("M00011"), func() {
			// Create SpiderMultusConfig and customize the net-attach-def name by annotating multus.spidernet.io/cr-name
			testSmc := smc.DeepCopy()
			testSmc.Name = "test-smc-3-" + common.GenerateString(10, true)
			sameNewSmcName := "test-another-smc-3-" + common.GenerateString(10, true)
			testSmc.ObjectMeta.Annotations = map[string]string{constant.AnnoNetAttachConfName: sameNewSmcName}
			GinkgoWriter.Printf("spidermultus cr with annotations: '%+v' \n", testSmc.Annotations)
			Expect(frame.CreateSpiderMultusInstance(testSmc)).NotTo(HaveOccurred())

			Eventually(func() bool {
				multusConfig, err := frame.GetMultusInstance(sameNewSmcName, testSmc.Namespace)
				GinkgoWriter.Printf("Auto-generated multus configuration %+v \n", multusConfig)
				return !api_errors.IsNotFound(err)
			}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())

			// Create another SpiderMultusConfig with the same name as the existing SpidermultusConfig's custom net-attach-def.
			newSmc := smc.DeepCopy()
			newSmc.Name = sameNewSmcName
			err := frame.CreateSpiderMultusInstance(newSmc)
			errorMsg := fmt.Sprintf("the net-attach-def %s/%s already exists and is managed by SpiderMultusConfig %s/%s.",
				newSmc.Namespace, sameNewSmcName, testSmc.Namespace, testSmc.Name)
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))
		})

		It("annotating custom names that are too long or empty should fail", Label("M00020"), func() {
			testSmc := smc.DeepCopy()
			longCustomizedName := common.GenerateString(k8svalidation.DNS1123SubdomainMaxLength+1, true)
			testSmc.ObjectMeta.Annotations = map[string]string{constant.AnnoNetAttachConfName: longCustomizedName}
			GinkgoWriter.Printf("spidermultus cr with annotations: '%+v' \n", testSmc.Annotations)
			err := frame.CreateSpiderMultusInstance(testSmc)
			errorMsg := fmt.Sprintf("must be no more than %d characters", k8svalidation.DNS1123SubdomainMaxLength)
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))

			emptyCustomizedName := ""
			testSmc.ObjectMeta.Annotations = map[string]string{constant.AnnoNetAttachConfName: emptyCustomizedName}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", testSmc.Annotations)
			err = frame.CreateSpiderMultusInstance(testSmc)
			errorMsg = "a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))
		})

		It("Change net-attach-conf version via annotation multus.spidernet.io/cni-version", Label("M00012"), func() {
			testSmc := smc.DeepCopy()
			cniVersion := "0.4.0"
			testSmc.ObjectMeta.Annotations = map[string]string{constant.AnnoMultusConfigCNIVersion: cniVersion}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", testSmc.Annotations)
			Expect(frame.CreateSpiderMultusInstance(testSmc)).NotTo(HaveOccurred())

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

		It("fail to customize unsupported CNI version", Label("M00013"), func() {
			mismatchCNIVersion := "x.y.z"
			smc.ObjectMeta.Annotations = map[string]string{constant.AnnoMultusConfigCNIVersion: mismatchCNIVersion}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", smc)
			// Mismatched versions, when doing a build, the error should occur here?
			Expect(frame.CreateSpiderMultusInstance(smc)).To(HaveOccurred())
		})

		It("The custom net-attach-conf name from the annotation multus.spidernet.io/cr-name doesn't follow Kubernetes naming rules and can't be created.", Label("M00025"), func() {
			testSmc := smc.DeepCopy()
			customNadName := "custom-error-name************"
			testSmc.ObjectMeta.Annotations = map[string]string{constant.AnnoNetAttachConfName: customNadName}
			GinkgoWriter.Printf("spidermultus cr with annotations: %+v \n", testSmc.Annotations)
			err := frame.CreateSpiderMultusInstance(testSmc)
			errorMsg := "a lowercase RFC 1123 subdomain must consist of lower case alphanumeric characters, '-' or '.', and must start and end with an alphanumeric character"
			Expect(err).To(MatchError(ContainSubstring(errorMsg)))
		})
	})

	It("Already have multus cr, spidermultus should take care of it", Label("M00014"), func() {
		var alreadyExistingNadName string = "already-multus-" + common.GenerateString(10, true)

		// Create a multus cr in advance
		nadObj := &v1.NetworkAttachmentDefinition{
			ObjectMeta: metav1.ObjectMeta{
				Name:      alreadyExistingNadName,
				Namespace: namespace,
			},
		}
		GinkgoWriter.Printf("multus cr: %+v \n", nadObj)
		err := frame.CreateMultusInstance(nadObj)
		Expect(err).NotTo(HaveOccurred())

		// Define spidermultus cr and create
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      alreadyExistingNadName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		Expect(frame.CreateSpiderMultusInstance(smc)).NotTo(HaveOccurred())

		Eventually(func() bool {
			multusConfig, err := frame.GetMultusInstance(alreadyExistingNadName, namespace)
			GinkgoWriter.Printf("Auto-generated multus configuration %+v \n", multusConfig)
			if api_errors.IsNotFound(err) {
				return false
			}
			// This value may be empty before managed by spidermultus
			if multusConfig.ObjectMeta.OwnerReferences == nil {
				return false
			}
			// The automatically generated multus configuration should be associated with spidermultus
			if multusConfig.ObjectMeta.OwnerReferences[0].Kind != constant.KindSpiderMultusConfig {
				return false
			}
			return true
		}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())
	})

	It("The value of webhook verification cniType is inconsistent with cniConf", Label("M00015"), func() {
		var smcName string = "multus-" + common.GenerateString(10, true)

		// Define Spidermultus cr where cniType does not agree with cniConf and create.
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.IPVlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		GinkgoWriter.Printf("should fail to create, the error is: %v", err.Error())
		Expect(err).To(HaveOccurred())
	})

	It("vlanID is not in the range of 0-4094 and will not be created", Label("M00016"), func() {
		var smcName string = "multus-" + common.GenerateString(10, true)

		// Define Spidermultus cr with vlanID -1
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
					VlanID: ptr.To(int32(-1)),
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		GinkgoWriter.Printf("should fail to create, the error is: %v \n", err.Error())
		Expect(err).To(HaveOccurred())

		// Define Spidermultus cr with vlanID 4095
		smc = &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
					VlanID: ptr.To(int32(4095)),
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err = frame.CreateSpiderMultusInstance(smc)
		GinkgoWriter.Printf("should fail to create, the error is: %v \n", err.Error())
		Expect(err).To(HaveOccurred())
	})

	It("testing creating spiderMultusConfig with cniType: ipvlan and checking the net-attach-conf config if works", Label("M00002"), func() {
		var smcName string = "ipvlan-" + common.GenerateString(10, true)

		// Define Spidermultus cr with ipvlan
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.IPVlanCNI),
				IPVlanConfig: &v2beta1.SpiderIPvlanCniConfig{
					Master: []string{common.NIC3},
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr with ipvlan: %+v \n", smc)
		Expect(frame.CreateSpiderMultusInstance(smc)).NotTo(HaveOccurred())

		Eventually(func() bool {
			ipvlanMultusConfig, err := frame.GetMultusInstance(smcName, namespace)
			GinkgoWriter.Printf("auto-generated ipvlan nad configuration %+v \n", ipvlanMultusConfig)
			if api_errors.IsNotFound(err) {
				return false
			}
			// The automatically generated multus configuration should be associated with spidermultus
			if ipvlanMultusConfig.ObjectMeta.OwnerReferences[0].Kind != constant.KindSpiderMultusConfig {
				return false
			}
			return true
		}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())
	})

	It("testing creating spiderMultusConfig with cniType: sriov and checking the net-attach-conf config if works", Label("M00003"), func() {
		var smcName string = "sriov-" + common.GenerateString(10, true)

		// Define Spidermultus cr with sriov
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.SriovCNI),
				SriovConfig: &v2beta1.SpiderSRIOVCniConfig{
					ResourceName: ptr.To("sriov-test"),
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr with sriov: %+v \n", smc)
		Expect(frame.CreateSpiderMultusInstance(smc)).NotTo(HaveOccurred())

		Eventually(func() bool {
			sriovMultusConfig, err := frame.GetMultusInstance(smcName, namespace)
			GinkgoWriter.Printf("auto-generated sriov nad configuration %+v \n", sriovMultusConfig)
			if api_errors.IsNotFound(err) {
				return false
			}
			// The automatically generated multus configuration should be associated with spidermultus
			if sriovMultusConfig.ObjectMeta.OwnerReferences[0].Kind != constant.KindSpiderMultusConfig {
				return false
			}
			return true
		}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())
	})

	It("testing creating spiderMultusConfig with cniType: custom and invalid/valid json config", Label("M00005", "M00004"), func() {
		var smcName string = "custom-multus" + common.GenerateString(10, true)

		invalidJson := `{ "invalid" }`
		// Define Spidermultus cr with invalid json config
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType:         ptr.To(constant.CustomCNI),
				CustomCNIConfig: &invalidJson,
			},
		}

		GinkgoWriter.Printf("spidermultus cr with invalid json config: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		GinkgoWriter.Printf("failed to create spidermultusconfig with invalid json config, error is %v", err)
		Expect(err).To(HaveOccurred())

		// Define valid json config
		validString := `{"cniVersion":"0.3.1","name":"macvlan-conf","plugins":[{"type":"macvlan","master":"eth0","mode":"bridge","ipam":{"type":"spiderpool",{"mode":"auto","type":"coordinator"}]}`
		validJson, err := json.Marshal(validString)
		Expect(err).NotTo(HaveOccurred())
		validJsonString := string(validJson)
		smc.Spec.CustomCNIConfig = &validJsonString
		GinkgoWriter.Printf("spidermultus cr with invalid json config: %+v \n", smc)
		Expect(frame.CreateSpiderMultusInstance(smc)).NotTo(HaveOccurred(), "failed to create spidermultusconfig with valid json config, error is %v", err)

		Eventually(func() bool {
			customMultusConfig, err := frame.GetMultusInstance(smcName, namespace)
			GinkgoWriter.Printf("auto-generated custom nad configuration %+v \n", customMultusConfig)
			if api_errors.IsNotFound(err) {
				return false
			}
			// The automatically generated multus configuration should be associated with spidermultus
			if customMultusConfig.ObjectMeta.OwnerReferences[0].Kind != constant.KindSpiderMultusConfig {
				return false
			}
			return true
		}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())
	})

	It("set spidermultusconfig.spec to empty and see if works", Label("M00019"), func() {
		smcName := "test-multus-" + common.GenerateString(10, true)
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
		}
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			netAttachDef, err := frame.GetMultusInstance(smcName, namespace)
			if nil != err {
				return err
			}
			if netAttachDef.Spec.Config != "" {
				return fmt.Errorf("empty SpiderMultusConfig %s/%s corresponding net-attach-def resource has CNI configuration: %s", smcName, namespace, netAttachDef.Spec.Config)
			}

			return nil
		}).WithTimeout(time.Minute * 3).WithPolling(time.Second * 5).Should(BeNil())
	})

	It("Update spidermultusConfig: add new bond config", Label("M00009", "M00006"), func() {
		smcName := "test-multus-" + common.GenerateString(10, true)
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{"eth0"},
				},
				EnableCoordinator: ptr.To(false),
			},
		}
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() error {
			netAttachDef, err := frame.GetMultusInstance(smcName, namespace)
			if nil != err {
				return err
			}
			if netAttachDef.Spec.Config == "" {
				return fmt.Errorf("SpiderMultusConfig %s/%s corresponding net-attach-def resource doesn't have CNI configuration", smcName, namespace)
			}

			configByte, err := netutils.GetCNIConfigFromSpec(netAttachDef.Spec.Config, netAttachDef.Name)
			if nil != err {
				return fmt.Errorf("GetCNIConfig: err in getCNIConfigFromSpec: %v", err)
			}

			networkConfigList, err := libcni.ConfListFromBytes(configByte)
			if nil != err {
				return err
			}

			if len(networkConfigList.Plugins) != 1 {
				return fmt.Errorf("unexpected CNI configuration: %s", netAttachDef.Spec.Config)
			}

			return nil
		}).WithTimeout(time.Minute * 2).WithPolling(time.Second * 5).Should(BeNil())

		// update the SpiderMultusConfig with bond
		smc.Spec.MacvlanConfig = &v2beta1.SpiderMacvlanCniConfig{
			Master: []string{"eth0", "eth1"},
			VlanID: ptr.To(int32(10)),
			Bond: &v2beta1.BondConfig{
				Name: "ens1",
				Mode: 0,
			},
		}
		GinkgoWriter.Printf("try to update SpiderMultusConfig with Bond configuration: %v", *smc.Spec.MacvlanConfig.Bond)
		err = frame.UpdateResource(smc)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			netAttachDef, err := frame.GetMultusInstance(smcName, namespace)
			if nil != err {
				return err
			}
			if netAttachDef.Spec.Config == "" {
				return fmt.Errorf("SpiderMultusConfig %s/%s corresponding net-attach-def resource doesn't have CNI configuration", smcName, namespace)
			}

			configByte, err := netutils.GetCNIConfigFromSpec(netAttachDef.Spec.Config, netAttachDef.Name)
			if nil != err {
				return fmt.Errorf("GetCNIConfig: err in getCNIConfigFromSpec: %v", err)
			}

			networkConfigList, err := libcni.ConfListFromBytes(configByte)
			if nil != err {
				return err
			}

			if len(networkConfigList.Plugins) != 2 {
				return fmt.Errorf("unexpected CNI configuration: %s", netAttachDef.Spec.Config)
			}

			GinkgoWriter.Printf("SpiderMultusConfig with Bond: %s\n", netAttachDef.Spec.Config)
			return nil
		}).WithTimeout(time.Minute * 3).WithPolling(time.Second * 5).Should(BeNil())
	})

	It("test hostRPFilter and podRPFilter in spiderMultusConfig", Label("M00022"), func() {
		var smcName string = "rpfilter-multus-" + common.GenerateString(10, true)

		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &v2beta1.CoordinatorSpec{
					PodRPFilter: nil,
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			netAttachDef, err := frame.GetMultusInstance(smcName, namespace)
			if nil != err {
				return err
			}
			if netAttachDef.Spec.Config == "" {
				return fmt.Errorf("SpiderMultusConfig %s/%s corresponding net-attach-def resource doesn't have CNI configuration", smcName, namespace)
			}

			configByte, err := netutils.GetCNIConfigFromSpec(netAttachDef.Spec.Config, netAttachDef.Name)
			if nil != err {
				return fmt.Errorf("GetCNIConfig: err in getCNIConfigFromSpec: %v", err)
			}

			networkConfigList, err := libcni.ConfListFromBytes(configByte)
			if nil != err {
				return err
			}

			if len(networkConfigList.Plugins) != 2 {
				return fmt.Errorf("unexpected CNI configuration: %s", netAttachDef.Spec.Config)
			}

			GinkgoWriter.Printf("SpiderMultusConfig with disableIPAM: %s\n", netAttachDef.Spec.Config)

			return nil
		}).WithTimeout(time.Minute * 3).WithPolling(time.Second * 5).Should(BeNil())
	})

	It("set podRPFilter to a invalid value", Label("M00023"), func() {
		var smcName string = "invalid-rpfilter-multus-" + common.GenerateString(10, true)

		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &v2beta1.CoordinatorSpec{
					PodRPFilter: ptr.To(4),
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).To(HaveOccurred(), "create spiderMultus instance failed: %v\n", err)
	})

	It("rdmaResourceName and ippools config must be set when spidermutlus with annotation: cni.spidernet.io/rdma-resource-inject", Label("M00027"), func() {
		var smcName string = "ann-rdma-resource" + common.GenerateString(10, true)
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
				Annotations: map[string]string{
					constant.AnnoPodResourceInject: "test",
				},
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master:           []string{common.NIC1},
					RdmaResourceName: ptr.To("test"),
					SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
						IPv4IPPool: []string{"test"},
					},
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &v2beta1.CoordinatorSpec{
					PodRPFilter: nil,
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).NotTo(HaveOccurred(), "create spiderMultus instance failed: %v\n", err)
	})

	It("return an err if no ippools config when spidermutlus with annotation: cni.spidernet.io/rdma-resource-inject", Label("M00029"), func() {
		var smcName string = "ann-rdma-resource" + common.GenerateString(10, true)
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
				Annotations: map[string]string{
					constant.AnnoPodResourceInject: "test",
				},
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master:           []string{common.NIC1},
					RdmaResourceName: ptr.To("test"),
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &v2beta1.CoordinatorSpec{
					PodRPFilter: nil,
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).To(HaveOccurred())
	})

	It("return an err if cniType is not in [macvlan,ipvlan,sriov,ib-sriov,ipoib] when spidermutlus with annotation: cni.spidernet.io/rdma-resource-inject", Label("M00030"), func() {
		var smcName string = "ann-rdma-resource" + common.GenerateString(10, true)
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
				Annotations: map[string]string{
					constant.AnnoPodResourceInject: "test",
				},
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.OvsCNI),
				OvsConfig: &v2beta1.SpiderOvsCniConfig{
					BrName:   "test",
					DeviceID: "test",
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &v2beta1.CoordinatorSpec{
					PodRPFilter: nil,
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).To(HaveOccurred())
	})

	It("resoucename and ippools config must be both set when spidermutlus with annotation: cni.spidernet.io/network-resource-inject", Label("M00031"), func() {
		var smcName string = "ann-network-resource" + common.GenerateString(10, true)
		smc := &spiderpoolv2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
				Annotations: map[string]string{
					constant.AnnoNetworkResourceInject: "test",
				},
			},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
					Master:           []string{common.NIC1},
					EnableRdma:       true,
					RdmaResourceName: "test",
					SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
						IPv4IPPool: []string{"test"},
					},
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
					PodRPFilter: nil,
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).NotTo(HaveOccurred(), "create spiderMultus instance failed: %v\n", err)
	})

	It("return an err if resoucename is set without ippools config when spidermutlus with annotation: cni.spidernet.io/network-resource-inject", Label("M00032"), func() {
		var smcName string = "ann-network-resource" + common.GenerateString(10, true)
		smc := &spiderpoolv2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
				Annotations: map[string]string{
					constant.AnnoNetworkResourceInject: "test",
				},
			},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
					Master:           []string{common.NIC1},
					EnableRdma:       true,
					RdmaResourceName: "test",
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{
					PodRPFilter: nil,
				},
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).To(HaveOccurred())
	})

	It("set disableIPAM to true and see if multus's nad has ipam config", Label("M00017"), func() {
		var smcName string = "disable-ipam-multus-" + common.GenerateString(10, true)

		// Define Spidermultus cr with DisableIPAM=true
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
				},
				DisableIPAM: ptr.To(true),
			},
		}
		GinkgoWriter.Printf("spidermultus cr: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).NotTo(HaveOccurred())

		Eventually(func() error {
			netAttachDef, err := frame.GetMultusInstance(smcName, namespace)
			if nil != err {
				return err
			}
			if netAttachDef.Spec.Config == "" {
				return fmt.Errorf("SpiderMultusConfig %s/%s corresponding net-attach-def resource doesn't have CNI configuration", smcName, namespace)
			}

			configByte, err := netutils.GetCNIConfigFromSpec(netAttachDef.Spec.Config, netAttachDef.Name)
			if nil != err {
				return fmt.Errorf("GetCNIConfig: err in getCNIConfigFromSpec: %v", err)
			}

			networkConfigList, err := libcni.ConfListFromBytes(configByte)
			if nil != err {
				return err
			}

			if len(networkConfigList.Plugins) != 2 {
				return fmt.Errorf("unexpected CNI configuration: %s", netAttachDef.Spec.Config)
			}

			Expect(netAttachDef.Spec.Config).NotTo(ContainSubstring(`"type":"spiderpool"`))
			GinkgoWriter.Printf("SpiderMultusConfig with disableIPAM: %s\n", netAttachDef.Spec.Config)

			return nil
		}).WithTimeout(time.Minute * 3).WithPolling(time.Second * 5).Should(BeNil())
	})

	It("create a spidermultusconfig to verify chainCNI json config", Label("M00021"), func() {
		var smcName string = "chain-cni-multus" + common.GenerateString(10, true)

		invalidJson := `{ "invalid" }`
		// Define Spidermultus cr with invalid json config
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{"eth0"},
					SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
						IPv4IPPool: []string{"spiderpool-ipv4-ippool"},
					},
				},
				ChainCNIJsonData: []string{invalidJson},
			},
		}

		if frame.Info.IpV4Enabled {
			smc.Spec.MacvlanConfig.SpiderpoolConfigPools.IPv4IPPool = []string{"default-v4-ippool"}
		}
		if frame.Info.IpV6Enabled {
			smc.Spec.MacvlanConfig.SpiderpoolConfigPools.IPv6IPPool = []string{"default-v6-ippool"}
		}

		GinkgoWriter.Printf("spidermultus cr with invalid ChainCNIJsonData: %+v \n", smc)
		err := frame.CreateSpiderMultusInstance(smc)
		GinkgoWriter.Printf("failed to create spidermultusconfig with invalid ChainCNIJsonData, error is %v\n", err)
		Expect(err).To(HaveOccurred())

		ginkgo.By("test chainCNI with valid json but empty cni type")
		jsonNoType := "{\"sysctl\":{\"net.core.somaxconn\":\"4096\"}}"
		smc.Spec.ChainCNIJsonData = []string{jsonNoType}
		GinkgoWriter.Printf("spidermultus cr with empty cni type: %+v \n", smc)
		err = frame.CreateSpiderMultusInstance(smc)
		GinkgoWriter.Printf("failed to create spidermultusconfig with no cni type config: %v\n", err)
		Expect(err).To(HaveOccurred())

		ginkgo.By("test chainCNI with valid json but not a map type")
		jsonNoMapType := "[{\"type\":\"tuning\",\"sysctl\":{\"net.core.somaxconn\":\"4096\"}}]"
		smc.Spec.ChainCNIJsonData = []string{jsonNoMapType}
		GinkgoWriter.Printf("spidermultus cr with no cni type config: %+v \n", smc)
		err = frame.CreateSpiderMultusInstance(smc)
		GinkgoWriter.Printf("failed to create spidermultusconfig with no map type: %v\n", err)
		Expect(err).To(HaveOccurred())

		// Define valid json config
		ginkgo.By("define valid json config")
		validString := "{\"type\":\"tuning\",\"sysctl\":{\"net.core.somaxconn\":\"4096\"}}"

		var netConf *types.NetConf
		err = json.Unmarshal([]byte(validString), &netConf)
		Expect(err).NotTo(HaveOccurred())

		smc.Spec.ChainCNIJsonData = []string{validString}
		GinkgoWriter.Printf("spidermultus cr with valid ChainCNIJsonData: %+v \n", smc)
		Expect(frame.CreateSpiderMultusInstance(smc)).NotTo(HaveOccurred(), "failed to create spidermultusconfig with valid ChainCNIJsonData: %v", err)

		Eventually(func() bool {
			config, err := frame.GetMultusInstance(smcName, namespace)
			GinkgoWriter.Printf("auto-generated custom nad configuration %+v \n", config)
			if api_errors.IsNotFound(err) {
				return false
			}

			// The automatically generated multus configuration should be associated with spidermultus
			if config.ObjectMeta.OwnerReferences[0].Kind != constant.KindSpiderMultusConfig {
				return false
			}

			GinkgoWriter.Printf("config: %v\n", config.Spec.Config)
			return !strings.Contains(config.Spec.Config, validString)
		}, common.SpiderSyncMultusTime, common.ForcedWaitingTime).Should(BeTrue())

		// create a pod
		var annotations = make(map[string]string)
		annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", namespace, smcName)
		deployObject := common.GenerateExampleDeploymentYaml(smcName, namespace, int32(1))
		deployObject.Spec.Template.Annotations = annotations
		Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()

		depObject, err := frame.WaitDeploymentReady(smcName, namespace, ctx)
		Expect(err).NotTo(HaveOccurred(), "waiting for deploy ready failed:  %v ", err)
		podList, err := frame.GetPodListByLabel(depObject.Spec.Template.Labels)
		Expect(err).NotTo(HaveOccurred(), "failed to get podList: %v ", err)

		commandString := "sysctl net.core.somaxconn | awk -F '=' '{print $2}' | tr -d ' '"
		ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		defer cancel()

		data, err := frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, commandString, ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to execute command, error is: %v ", err)

		Expect(string(data)).To(Equal("4096\n"), "net.core.somaxconn: %s", data)
	})

	It("check the enableVethLinkLocakAddress works", Label("M00026"), func() {
		// create a pod
		name := "veth-address-test"
		var annotations = make(map[string]string)
		annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)
		deployObject := common.GenerateExampleDeploymentYaml("veth-address-test", namespace, int32(1))
		deployObject.Spec.Template.Annotations = annotations
		Expect(frame.CreateDeployment(deployObject)).NotTo(HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()

		depObject, err := frame.WaitDeploymentReady(name, namespace, ctx)
		Expect(err).NotTo(HaveOccurred(), "waiting for deploy ready failed:  %v ", err)
		podList, err := frame.GetPodListByLabel(depObject.Spec.Template.Labels)
		Expect(err).NotTo(HaveOccurred(), "failed to get podList: %v ", err)

		commandString := "ip a show veth0 | grep 169.254.200.1 &> /dev/null"
		ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		defer cancel()

		_, err = frame.ExecCommandInPod(podList.Items[0].Name, podList.Items[0].Namespace, commandString, ctx)
		Expect(err).NotTo(HaveOccurred(), "failed to execute command, err: %v ", err)
	})

	It("verify the podMACPrefix filed", Label("M00024"), func() {
		smcName := "test-multus-" + common.GenerateString(10, true)
		smc := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      smcName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{"eth0"},
					SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
						IPv4IPPool: []string{"spiderpool-ipv4-ippool"},
					},
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &v2beta1.CoordinatorSpec{
					PodMACPrefix: ptr.To("9e:10"),
				},
			},
		}

		ginkgo.By("create a spiderMultusConfig with valid podMACPrefix")
		err := frame.CreateSpiderMultusInstance(smc)
		Expect(err).NotTo(HaveOccurred())

		tmpName := "invalid-macprefix" + common.GenerateString(10, true)
		invalid := &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      tmpName,
				Namespace: namespace,
			},
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType: ptr.To(constant.MacvlanCNI),
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
					Master: []string{"eth0"},
					SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
						IPv4IPPool: []string{"spiderpool-ipv4-ippool"},
					},
				},
				EnableCoordinator: ptr.To(true),
				CoordinatorConfig: &v2beta1.CoordinatorSpec{
					PodMACPrefix: ptr.To("01:10"),
				},
			},
		}

		ginkgo.By("create a spiderMultusConfig with invalid podMACPrefix")
		err = frame.CreateSpiderMultusInstance(invalid)
		Expect(err).To(HaveOccurred(), "create invalid spiderMultusConfig should fail: %v", err)
	})
})
