// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
								RdmaResourceName: ptr.To("spidernet.io/rdma-resource1"),
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
								RdmaResourceName: ptr.To("spidernet.io/rdma-resource2"),
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
								RdmaResourceName: ptr.To("spidernet.io/rdma-resource1"),
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
								RdmaResourceName: ptr.To("spidernet.io/rdma-resource2"),
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
								RdmaResourceName: ptr.To("spidernet.io/rdma-resource1"),
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
								RdmaResourceName: ptr.To("spidernet.io/rdma-resource2"),
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
							RdmaResourceName: ptr.To("rdma-resource"),
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
							RdmaResourceName: ptr.To(""),
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
							RdmaResourceName: ptr.To("rdma-resource"),
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
							RdmaResourceName: ptr.To(""),
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
							ResourceName: ptr.To("rdma-resource"),
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
							ResourceName: ptr.To(""),
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
							ResourceName: ptr.To("rdma-resource"),
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
							ResourceName: ptr.To(""),
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
