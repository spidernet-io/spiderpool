// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/podmanager"
)

var _ = Describe("PodManager utils", Label("pod_manager_utils_test"), func() {
	var podT *corev1.Pod

	BeforeEach(func() {
		podT = &corev1.Pod{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Pod",
				APIVersion: corev1.SchemeGroupVersion.String(),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod",
				Namespace: "default",
			},
			Spec:   corev1.PodSpec{},
			Status: corev1.PodStatus{},
		}
	})

	Describe("Test IsPodAlive", func() {
		It("inputs nil Pod", func() {
			isAlive := podmanager.IsPodAlive(nil)
			Expect(isAlive).To(BeFalse())
		})

		It("checks terminating Pod", func() {
			now := metav1.Now()
			podT.SetDeletionTimestamp(&now)
			podT.SetDeletionGracePeriodSeconds(ptr.To(int64(30)))

			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeFalse())
		})

		It("checks succeeded Pod", func() {
			podT.Status.Phase = corev1.PodSucceeded
			podT.Spec.RestartPolicy = corev1.RestartPolicyNever

			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeFalse())
		})

		It("checks failed Pod", func() {
			podT.Status.Phase = corev1.PodFailed
			podT.Spec.RestartPolicy = corev1.RestartPolicyNever

			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeFalse())
		})

		It("checks evicted Pod", func() {
			podT.Status.Phase = corev1.PodFailed
			podT.Status.Reason = "Evicted"

			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeFalse())
		})

		It("checks running Pod", func() {
			isAlive := podmanager.IsPodAlive(podT)
			Expect(isAlive).To(BeTrue())
		})
	})

	Describe("Test injectPodNetwork", Label("inject_pod_network_test"), func() {
		var pod *corev1.Pod
		var multusConfigs v2beta1.SpiderMultusConfigList

		BeforeEach(func() {
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "test-pod",
					Namespace:   "default",
					Annotations: make(map[string]string),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "test-container",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{},
								Limits:   corev1.ResourceList{},
							},
						},
					},
				},
			}
		})

		It("should successfully inject network configuration", func() {
			multusConfigs = v2beta1.SpiderMultusConfigList{
				Items: []v2beta1.SpiderMultusConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "config1",
							Namespace: "default",
						},
						Spec: v2beta1.MultusCNIConfigSpec{
							CniType: ptr.To("macvlan"),
							MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
								EnableRdma:       true,
								RdmaResourceName: "spidernet.io/rdma-resource1",
								SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
									IPv4IPPool: []string{"test1"},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "config2",
							Namespace: "default",
						},
						Spec: v2beta1.MultusCNIConfigSpec{
							CniType: ptr.To("macvlan"),
							MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
								EnableRdma:       true,
								RdmaResourceName: "spidernet.io/rdma-resource2",
								SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
									IPv4IPPool: []string{"test1"},
								},
							},
						},
					},
				},
			}
			err := podmanager.InjectPodNetwork(pod, multusConfigs)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Annotations[constant.MultusNetworkAttachmentAnnot]).To(Equal("default/config1,default/config2"))

			Expect(pod.Spec.Containers[0].Resources.Limits).To(HaveKey(corev1.ResourceName("spidernet.io/rdma-resource1")))
			Expect(pod.Spec.Containers[0].Resources.Limits).To(HaveKey(corev1.ResourceName("spidernet.io/rdma-resource2")))
		})

		It("should return an error when no ippools configured", func() {
			multusConfigs = v2beta1.SpiderMultusConfigList{
				Items: []v2beta1.SpiderMultusConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "config1",
							Namespace: "default",
						},
						Spec: v2beta1.MultusCNIConfigSpec{
							CniType: ptr.To("macvlan"),
							MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
								EnableRdma:       true,
								RdmaResourceName: "spidernet.io/rdma-resource1",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "config2",
							Namespace: "default",
						},
						Spec: v2beta1.MultusCNIConfigSpec{
							CniType: ptr.To("macvlan"),
							MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
								EnableRdma:       true,
								RdmaResourceName: "spidernet.io/rdma-resource2",
								SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
									IPv4IPPool: []string{"test1"},
								},
							},
						},
					},
				},
			}
			err := podmanager.InjectPodNetwork(pod, multusConfigs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("No any ippools configured"))
		})

		It("should return an error when not disable rdma", func() {
			multusConfigs = v2beta1.SpiderMultusConfigList{
				Items: []v2beta1.SpiderMultusConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "config1",
							Namespace: "default",
						},
						Spec: v2beta1.MultusCNIConfigSpec{
							CniType: ptr.To("macvlan"),
							MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
								EnableRdma:       false,
								RdmaResourceName: "spidernet.io/rdma-resource1",
								SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
									IPv4IPPool: []string{"test1"},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "config2",
							Namespace: "default",
						},
						Spec: v2beta1.MultusCNIConfigSpec{
							CniType: ptr.To("macvlan"),
							MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
								EnableRdma:       true,
								RdmaResourceName: "spidernet.io/rdma-resource2",
								SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
									IPv4IPPool: []string{"test1"},
								},
							},
						},
					},
				},
			}
			err := podmanager.InjectPodNetwork(pod, multusConfigs)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not enable RDMA"))
		})

		It("should preserve existing resources in the Pod", func() {
			// Set some pre-existing resources
			pod.Spec.Containers[0].Resources.Limits = corev1.ResourceList{
				corev1.ResourceName("spidernet.io/rdma-resource1"): resource.MustParse("1"),
				corev1.ResourceName("existing-resource"):           resource.MustParse("10"),
			}

			multusConfigs = v2beta1.SpiderMultusConfigList{
				Items: []v2beta1.SpiderMultusConfig{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "config1",
							Namespace: "default",
						},
						Spec: v2beta1.MultusCNIConfigSpec{
							CniType: ptr.To("macvlan"),
							MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
								EnableRdma:       true,
								RdmaResourceName: "spidernet.io/rdma-resource1",
								SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
									IPv4IPPool: []string{"test1"},
								},
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "config2",
							Namespace: "default",
						},
						Spec: v2beta1.MultusCNIConfigSpec{
							CniType: ptr.To("macvlan"),
							MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
								EnableRdma:       true,
								RdmaResourceName: "spidernet.io/rdma-resource2",
								SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
									IPv4IPPool: []string{"test1"},
								},
							},
						},
					},
				},
			}

			err := podmanager.InjectPodNetwork(pod, multusConfigs)
			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Annotations[constant.MultusNetworkAttachmentAnnot]).To(Equal("default/config1,default/config2"))

			// Verify that existing resources are preserved
			Expect(pod.Spec.Containers[0].Resources.Limits).To(HaveKey(corev1.ResourceName("spidernet.io/rdma-resource1")))
			Expect(pod.Spec.Containers[0].Resources.Limits).To(HaveKey(corev1.ResourceName("spidernet.io/rdma-resource2")))
			Expect(pod.Spec.Containers[0].Resources.Limits).To(HaveKey(corev1.ResourceName("existing-resource")))
			Expect(pod.Spec.Containers[0].Resources.Limits[corev1.ResourceName("existing-resource")]).To(Equal(resource.MustParse("10")))
		})
	})

	Describe("Utils", func() {
		Context("initPodMutatingWebhook", func() {
			It("should properly initialize pod mutating webhook with full configuration", func() {
				// Prepare test data
				testCABundle := []byte("test-ca-bundle")
				fromWebhook := admissionregistrationv1.MutatingWebhook{
					AdmissionReviewVersions: []string{"v1", "v1beta1"},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						CABundle: testCABundle,
						Service: &admissionregistrationv1.ServiceReference{
							Name:      "test-service",
							Namespace: "test-namespace",
							Port:      ptr.To(int32(443)),
						},
					},
				}

				// Call the function under test
				podWebhookNamespaceInclude := []string{
					"test",
				}
				result := podmanager.InitPodMutatingWebhook(fromWebhook, podWebhookNamespaceInclude)

				// Verify results
				Expect(result.Name).To(Equal(constant.PodMutatingWebhookName))
				Expect(result.AdmissionReviewVersions).To(Equal(fromWebhook.AdmissionReviewVersions))
				Expect(*result.FailurePolicy).To(Equal(admissionregistrationv1.Fail))

				// Verify NamespaceSelector
				Expect(result.NamespaceSelector).NotTo(BeNil())
				Expect(result.NamespaceSelector.MatchExpressions).To(HaveLen(2))
				Expect(result.NamespaceSelector.MatchExpressions[0].Key).To(Equal(corev1.LabelMetadataName))
				Expect(result.NamespaceSelector.MatchExpressions[0].Operator).To(Equal(metav1.LabelSelectorOpNotIn))
				Expect(result.NamespaceSelector.MatchExpressions[1].Key).To(Equal(corev1.LabelMetadataName))
				Expect(result.NamespaceSelector.MatchExpressions[1].Operator).To(Equal(metav1.LabelSelectorOpIn))

				// Verify ClientConfig
				Expect(result.ClientConfig.CABundle).To(Equal(testCABundle))
				Expect(result.ClientConfig.Service).NotTo(BeNil())
				Expect(result.ClientConfig.Service.Name).To(Equal("test-service"))
				Expect(result.ClientConfig.Service.Namespace).To(Equal("test-namespace"))
				Expect(*result.ClientConfig.Service.Port).To(Equal(int32(443)))
				Expect(*result.ClientConfig.Service.Path).To(Equal("/mutate--v1-pod"))

				// Verify Rules
				Expect(result.Rules).To(HaveLen(1))
				Expect(result.Rules[0].Operations).To(ConsistOf(
					admissionregistrationv1.Create,
					admissionregistrationv1.Update,
				))
				Expect(result.Rules[0].Rule.APIGroups).To(Equal([]string{""}))
				Expect(result.Rules[0].Rule.APIVersions).To(Equal([]string{"v1"}))
				Expect(result.Rules[0].Rule.Resources).To(Equal([]string{"pods"}))

				// Verify SideEffects
				Expect(*result.SideEffects).To(Equal(admissionregistrationv1.SideEffectClassNone))
			})

			It("should properly initialize webhook without Service configuration", func() {
				// Prepare test data
				fromWebhook := admissionregistrationv1.MutatingWebhook{
					AdmissionReviewVersions: []string{"v1"},
					ClientConfig: admissionregistrationv1.WebhookClientConfig{
						CABundle: []byte("test-ca-bundle"),
					},
				}

				// Call the function under test
				result := podmanager.InitPodMutatingWebhook(fromWebhook, []string{})

				// Verify results
				Expect(result.ClientConfig.Service).To(BeNil())
				Expect(result.Name).To(Equal(constant.PodMutatingWebhookName))
			})
		})
	})

	Describe("AddPodMutatingWebhook", func() {
		var (
			fakeClient                 *fake.Clientset
			webhookName                string
			existingConfig             *admissionregistrationv1.MutatingWebhookConfiguration
			podWebhookNamespaceInclude []string
		)

		BeforeEach(func() {
			// Initialize test variables
			fakeClient = fake.NewSimpleClientset()
			webhookName = "test-webhook-config"
			podWebhookNamespaceInclude = []string{
				"test",
			}

			// Create a basic webhook configuration
			existingConfig = &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: webhookName,
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{
						Name: "existing-webhook",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							CABundle: []byte("test-ca-bundle"),
							Service: &admissionregistrationv1.ServiceReference{
								Name:      "webhook-service",
								Namespace: "default",
								Port:      ptr.To(int32(443)),
							},
						},
						AdmissionReviewVersions: []string{"v1"},
					},
				},
			}
		})

		Context("when adding pod mutating webhook", func() {
			It("should successfully add webhook when it doesn't exist", func() {
				// Create initial webhook configuration
				_, err := fakeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(
					context.TODO(), existingConfig, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Call the function under test
				err = podmanager.AddPodMutatingWebhook(fakeClient.AdmissionregistrationV1(), webhookName, podWebhookNamespaceInclude)
				Expect(err).NotTo(HaveOccurred())

				// // Verify the webhook was added
				updatedConfig, err := fakeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(
					context.TODO(), webhookName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedConfig.Webhooks).To(HaveLen(2))
				Expect(updatedConfig.Webhooks[1].Name).To(Equal(constant.PodMutatingWebhookName))
			})

			It("should not add webhook when it already exists", func() {
				// Add pod webhook to initial configuration
				podWebhook := podmanager.InitPodMutatingWebhook(existingConfig.Webhooks[0], podWebhookNamespaceInclude)
				existingConfig.Webhooks = append(existingConfig.Webhooks, podWebhook)

				// Create webhook configuration with pod webhook
				_, err := fakeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(
					context.TODO(), existingConfig, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Call the function under test
				err = podmanager.AddPodMutatingWebhook(fakeClient.AdmissionregistrationV1(), webhookName, podWebhookNamespaceInclude)
				Expect(err).NotTo(HaveOccurred())

				// // Verify no additional webhook was added
				updatedConfig, err := fakeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(
					context.TODO(), webhookName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedConfig.Webhooks).To(HaveLen(2))
			})

			It("should return error when webhook configuration doesn't exist", func() {
				// Call the function under test without creating webhook configuration
				err := podmanager.AddPodMutatingWebhook(fakeClient.AdmissionregistrationV1(), webhookName, podWebhookNamespaceInclude)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to get MutatingWebhookConfiguration"))
			})

			It("should return error when webhook configuration is empty", func() {
				// Create empty webhook configuration
				emptyConfig := &admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: webhookName,
					},
				}
				_, err := fakeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(
					context.TODO(), emptyConfig, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Call the function under test
				err = podmanager.AddPodMutatingWebhook(fakeClient.AdmissionregistrationV1(), webhookName, podWebhookNamespaceInclude)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no any mutating webhook found"))
			})
		})
	})

	var _ = Describe("RemovePodMutatingWebhook", func() {
		var (
			// Mock admission client
			fakeClient *fake.Clientset
			// Test webhook name
			webhookName string
		)

		BeforeEach(func() {
			// Initialize test variables
			// Initialize test variables
			fakeClient = fake.NewSimpleClientset()
			webhookName = "test-webhook-config"
		})

		Context("when removing pod mutating webhook", func() {
			It("should successfully remove the webhook if it exists", func() {
				// Prepare existing webhook configuration
				existingWebhooks := []admissionregistrationv1.MutatingWebhook{
					{Name: constant.PodMutatingWebhookName},
					{Name: "other-webhook"},
				}

				mwc := &admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: webhookName,
					},
					Webhooks: existingWebhooks,
				}

				// Setup mock behavior
				_, err := fakeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(
					context.TODO(), mwc, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Execute test
				err = podmanager.RemovePodMutatingWebhook(fakeClient.AdmissionregistrationV1(), webhookName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return nil if webhook doesn't exist", func() {
				// Prepare existing webhook configuration
				existingWebhooks := []admissionregistrationv1.MutatingWebhook{
					{Name: "other-webhook"},
				}

				mwc := &admissionregistrationv1.MutatingWebhookConfiguration{
					ObjectMeta: metav1.ObjectMeta{
						Name: webhookName,
					},
					Webhooks: existingWebhooks,
				}

				// Setup mock behavior
				_, err := fakeClient.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(
					context.TODO(), mwc, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())

				// Execute test
				err = podmanager.RemovePodMutatingWebhook(fakeClient.AdmissionregistrationV1(), webhookName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return error when getting webhook configuration fails", func() {
				err := podmanager.RemovePodMutatingWebhook(fakeClient.AdmissionregistrationV1(), webhookName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})
	})

	Describe("DoValidateRdmaResouce", func() {
		var (
			mc v2beta1.SpiderMultusConfig
		)

		Context("when CNI type is Macvlan", func() {
			It("should not return an error for valid RDMA configuration", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.MacvlanCNI),
						MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
							EnableRdma:       true,
							RdmaResourceName: "rdma-resource",
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test"},
							},
						},
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).To(BeNil())
			})

			It("should return an error for invalid RDMA configuration", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.MacvlanCNI),
						MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
							EnableRdma:       false,
							RdmaResourceName: "",
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test"},
							},
						},
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when CNI type is ipvlan", func() {
			It("should not return an error for valid RDMA configuration", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.IPVlanCNI),
						IPVlanConfig: &v2beta1.SpiderIPvlanCniConfig{
							EnableRdma:       true,
							RdmaResourceName: "rdma-resource",
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test"},
							},
						},
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).To(BeNil())
			})

			It("should return an error for invalid RDMA configuration", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.IPVlanCNI),
						IPVlanConfig: &v2beta1.SpiderIPvlanCniConfig{
							EnableRdma:       false,
							RdmaResourceName: "",
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test"},
							},
						},
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when CNI type is sriov", func() {
			It("should not return an error for valid RDMA configuration", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.SriovCNI),
						SriovConfig: &v2beta1.SpiderSRIOVCniConfig{
							EnableRdma:   true,
							ResourceName: "rdma-resource",
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test"},
							},
						},
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).To(BeNil())
			})

			It("should return an error for invalid RDMA configuration", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.SriovCNI),
						SriovConfig: &v2beta1.SpiderSRIOVCniConfig{
							EnableRdma:   false,
							ResourceName: "",
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test"},
							},
						},
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when CNI type is ibsriov", func() {
			It("should not return an error for valid RDMA configuration", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.IBSriovCNI),
						IbSriovConfig: &v2beta1.SpiderIBSriovCniConfig{
							ResourceName: "rdma-resource",
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test"},
							},
						},
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).To(BeNil())
			})

			It("should return an error for invalid RDMA configuration", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.IBSriovCNI),
						IbSriovConfig: &v2beta1.SpiderIBSriovCniConfig{
							ResourceName: "",
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"test"},
							},
						},
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).NotTo(BeNil())
			})
		})

		Context("when CNI type is invalid", func() {
			It("should return an error", func() {
				mc = v2beta1.SpiderMultusConfig{
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To("ovs"),
					},
				}
				err := podmanager.DoValidateRdmaResouce(mc)
				Expect(err).NotTo(BeNil())
			})
		})
	})
})
