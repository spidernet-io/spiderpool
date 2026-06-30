// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package podmanager

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	spiderpoolfake "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type stubNamespaceManager struct {
	namespace  *corev1.Namespace
	err        error
	callCount  int
	lastName   string
	lastCached bool
}

func (s *stubNamespaceManager) GetNamespaceByName(ctx context.Context, nsName string, cached bool) (*corev1.Namespace, error) {
	s.callCount++
	s.lastName = nsName
	s.lastCached = cached
	if s.err != nil {
		return nil, s.err
	}
	return s.namespace, nil
}

func (s *stubNamespaceManager) ListNamespaces(ctx context.Context, cached bool, opts ...client.ListOption) (*corev1.NamespaceList, error) {
	return nil, nil
}

var _ = Describe("Pod Webhook Internal", Label("podwebhook", "unittest"), func() {
	Describe("PWebhook validate stubs", func() {
		var pw *PWebhook

		BeforeEach(func() {
			pw = &PWebhook{spiderClient: spiderpoolfake.NewSimpleClientset()}
		})

		It("ValidateCreate returns no warnings and no error", func() {
			pod := &corev1.Pod{}
			warnings, err := pw.ValidateCreate(context.Background(), pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("ValidateUpdate returns no warnings and no error", func() {
			pod := &corev1.Pod{}
			warnings, err := pw.ValidateUpdate(context.Background(), pod, pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("ValidateDelete returns no warnings and no error", func() {
			pod := &corev1.Pod{}
			warnings, err := pw.ValidateDelete(context.Background(), pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(warnings).To(BeNil())
		})

		It("Default is a no-op for a pod with no relevant annotations", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "default"},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			}
			Expect(pw.Default(context.Background(), pod)).To(Succeed())
		})
	})

	Describe("isProviderVLANSpiderMultusConfig", func() {
		It("returns false for nil input", func() {
			Expect(isProviderVLANSpiderMultusConfig(nil)).To(BeFalse())
		})

		It("returns false when CniType is nil", func() {
			Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{})).To(BeFalse())
		})

		It("returns false for a non-VlanCNI type", func() {
			cniType := constant.MacvlanCNI
			Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{
				Spec: v2beta1.MultusCNIConfigSpec{CniType: &cniType},
			})).To(BeFalse())
		})

		It("returns false when VlanConfig is nil", func() {
			cniType := constant.VlanCNI
			Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{
				Spec: v2beta1.MultusCNIConfigSpec{CniType: &cniType},
			})).To(BeFalse())
		})

		It("returns false when VlanMode is manual", func() {
			cniType := constant.VlanCNI
			vlanID := ptr.To[int32](100)
			Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:    &cniType,
					VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeManual), VlanID: vlanID},
				},
			})).To(BeFalse())
		})

		It("returns false when VlanMode is nil", func() {
			cniType := constant.VlanCNI
			Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:    &cniType,
					VlanConfig: &v2beta1.SpiderVlanCniConfig{},
				},
			})).To(BeFalse())
		})

		It("returns true for a VlanCNI config with VlanMode auto", func() {
			cniType := constant.VlanCNI
			Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:    &cniType,
					VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeAuto)},
				},
			})).To(BeTrue())
		})
	})

	Describe("podHasResource", func() {
		It("returns false for a nil pod", func() {
			Expect(podHasResource(nil, constant.DefaultENISlotResourceName)).To(BeFalse())
		})

		It("returns false when the pod has no containers", func() {
			pod := &corev1.Pod{}
			Expect(podHasResource(pod, constant.DefaultENISlotResourceName)).To(BeFalse())
		})

		It("returns true when the resource exists in Limits", func() {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Resources: corev1.ResourceRequirements{
							Limits: corev1.ResourceList{
								corev1.ResourceName(constant.DefaultENISlotResourceName): resource.MustParse("1"),
							},
						},
					}},
				},
			}
			Expect(podHasResource(pod, constant.DefaultENISlotResourceName)).To(BeTrue())
		})

		It("returns true when the resource exists in Requests", func() {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceName(constant.DefaultENISlotResourceName): resource.MustParse("1"),
							},
						},
					}},
				},
			}
			Expect(podHasResource(pod, constant.DefaultENISlotResourceName)).To(BeTrue())
		})
	})

	Describe("getEffectiveResourceInjectValue", func() {
		var (
			ctx       context.Context
			nsManager *stubNamespaceManager
		)

		BeforeEach(func() {
			ctx = context.Background()
			nsManager = &stubNamespaceManager{
				namespace: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: "tenant-a",
						Annotations: map[string]string{
							constant.AnnoNetworkResourceInject: "vlan100",
						},
					},
				},
			}
		})

		Context("when Pod defines the annotation", func() {
			It("should return the Pod annotation value without consulting the Namespace", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-a",
						Namespace: "tenant-a",
						Annotations: map[string]string{
							constant.AnnoNetworkResourceInject: "vlan200",
						},
					},
				}

				value, ok, err := getEffectiveResourceInjectValue(ctx, nsManager, pod, constant.AnnoNetworkResourceInject)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(value).To(Equal("vlan200"))
				Expect(nsManager.callCount).To(Equal(0), "namespace manager should not be called when Pod has annotation")
			})
		})

		Context("when Pod does not define the annotation", func() {
			It("should fall back to the Namespace annotation via cached read", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-a",
						Namespace: "tenant-a",
					},
				}

				value, ok, err := getEffectiveResourceInjectValue(ctx, nsManager, pod, constant.AnnoNetworkResourceInject)
				Expect(err).NotTo(HaveOccurred())
				Expect(ok).To(BeTrue())
				Expect(value).To(Equal("vlan100"))
				Expect(nsManager.callCount).To(Equal(1), "namespace manager should be called exactly once")
				Expect(nsManager.lastName).To(Equal("tenant-a"))
				Expect(nsManager.lastCached).To(BeTrue(), "namespace lookup should use cache")
			})
		})
	})

	Describe("podENIResourceMutatingWebhook", Label("podwebhook_eni_resource_test"), func() {
		It("should inject ENI resources for eligible VLAN SpiderMultusConfigs from default and attachment annotations", func() {
			ctx := context.Background()
			cniType := constant.VlanCNI
			spiderClient := spiderpoolfake.NewSimpleClientset(
				&v2beta1.SpiderMultusConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "default-net", Namespace: "tenant-a"},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType:    &cniType,
						VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeAuto)},
					},
				},
				&v2beta1.SpiderMultusConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "attach-net", Namespace: "tenant-a"},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType:    &cniType,
						VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeAuto)},
					},
				},
			)
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-a",
					Namespace: "tenant-a",
					Annotations: map[string]string{
						constant.MultusDefaultNetAnnot:        "tenant-a/default-net",
						constant.MultusNetworkAttachmentAnnot: "tenant-a/attach-net",
					},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			}

			err := podENIResourceMutatingWebhook(ctx, spiderClient, pod, PodENIResourceInjectConfig{
				ProviderEnabled:       true,
				PluginEnabled:         true,
				ResourceName:          constant.DefaultENISlotResourceName,
				InjectPodENIResources: true,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers[0].Resources.Limits[corev1.ResourceName(constant.DefaultENISlotResourceName)]).To(Equal(resource.MustParse("2")))
			Expect(pod.Spec.Containers[0].Resources.Requests[corev1.ResourceName(constant.DefaultENISlotResourceName)]).To(Equal(resource.MustParse("2")))
		})

		It("should skip ENI injection when provider mode is disabled", func() {
			ctx := context.Background()
			cniType := constant.VlanCNI
			spiderClient := spiderpoolfake.NewSimpleClientset(&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "net-a", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:    &cniType,
					VlanConfig: &v2beta1.SpiderVlanCniConfig{},
				},
			})
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod-a",
					Namespace:   "tenant-a",
					Annotations: map[string]string{constant.MultusNetworkAttachmentAnnot: "tenant-a/net-a"},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			}

			err := podENIResourceMutatingWebhook(ctx, spiderClient, pod, PodENIResourceInjectConfig{
				ProviderEnabled:       false,
				PluginEnabled:         true,
				ResourceName:          constant.DefaultENISlotResourceName,
				InjectPodENIResources: true,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers[0].Resources.Limits).NotTo(HaveKey(corev1.ResourceName(constant.DefaultENISlotResourceName)))
		})

		It("should skip ENI injection when the device plugin is disabled", func() {
			ctx := context.Background()
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod-a",
					Namespace:   "tenant-a",
					Annotations: map[string]string{constant.MultusNetworkAttachmentAnnot: "tenant-a/net-a"},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			}

			err := podENIResourceMutatingWebhook(ctx, spiderpoolfake.NewSimpleClientset(), pod, PodENIResourceInjectConfig{
				ProviderEnabled:       true,
				PluginEnabled:         false,
				ResourceName:          constant.DefaultENISlotResourceName,
				InjectPodENIResources: true,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers[0].Resources.Limits).NotTo(HaveKey(corev1.ResourceName(constant.DefaultENISlotResourceName)))
		})
	})

	Describe("podMasterNICResourceMutatingWebhook", Label("podwebhook_master_nic_resource_test"), func() {
		It("should inject each distinct master NIC resource referenced by the Pod", func() {
			ctx := context.Background()
			macvlanType := constant.MacvlanCNI
			vlanType := constant.VlanCNI
			ipoibType := constant.IPoIBCNI
			spiderClient := spiderpoolfake.NewSimpleClientset(
				&v2beta1.SpiderMultusConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "default-net", Namespace: "tenant-a"},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: &macvlanType,
						MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{
							Master: []string{"eth1", "eth2"},
							Bond:   &v2beta1.BondConfig{Name: "bond0"},
						},
					},
				},
				&v2beta1.SpiderMultusConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "attach-net", Namespace: "tenant-a"},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType:    &vlanType,
						VlanConfig: &v2beta1.SpiderVlanCniConfig{Master: []string{"eth1"}},
					},
				},
				&v2beta1.SpiderMultusConfig{
					ObjectMeta: metav1.ObjectMeta{Name: "ipoib-net", Namespace: "tenant-a"},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType:     &ipoibType,
						IpoibConfig: &v2beta1.SpiderIpoibCniConfig{Master: "ib0"},
					},
				},
			)
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-a",
					Namespace: "tenant-a",
					Annotations: map[string]string{
						constant.MultusDefaultNetAnnot:        "tenant-a/default-net",
						constant.MultusNetworkAttachmentAnnot: "tenant-a/attach-net,tenant-a/ipoib-net",
					},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			}

			err := podMasterNICResourceMutatingWebhook(ctx, spiderClient, pod, PodENIResourceInjectConfig{
				MasterNICEnabled:      true,
				InjectPodENIResources: true,
			})

			Expect(err).NotTo(HaveOccurred())
			for _, resourceName := range []corev1.ResourceName{
				"spidernet.io/eth1-nic",
				"spidernet.io/eth2-nic",
				"spidernet.io/ib0-nic",
			} {
				Expect(pod.Spec.Containers[0].Resources.Limits[resourceName]).To(Equal(resource.MustParse("1")))
				Expect(pod.Spec.Containers[0].Resources.Requests[resourceName]).To(Equal(resource.MustParse("1")))
			}
			Expect(pod.Spec.Containers[0].Resources.Limits).To(HaveLen(3))
		})

		It("should preserve a master NIC resource already declared by the workload", func() {
			ctx := context.Background()
			cniType := constant.MacvlanCNI
			resourceName := corev1.ResourceName("spidernet.io/eth1-nic")
			spiderClient := spiderpoolfake.NewSimpleClientset(&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "net-a", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:       &cniType,
					MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{Master: []string{"eth1"}},
				},
			})
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod-a",
					Namespace:   "tenant-a",
					Annotations: map[string]string{constant.MultusNetworkAttachmentAnnot: "tenant-a/net-a"},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{
					Name: "app",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{resourceName: resource.MustParse("2")},
					},
				}}},
			}

			err := podMasterNICResourceMutatingWebhook(ctx, spiderClient, pod, PodENIResourceInjectConfig{
				MasterNICEnabled:      true,
				InjectPodENIResources: true,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers[0].Resources.Limits[resourceName]).To(Equal(resource.MustParse("2")))
			Expect(pod.Spec.Containers[0].Resources.Requests).NotTo(HaveKey(resourceName))
		})

		It("should ignore referenced networks that are not backed by SpiderMultusConfig", func() {
			ctx := context.Background()
			cniType := constant.IPVlanCNI
			resourceName := corev1.ResourceName("spidernet.io/eth1-nic")
			spiderClient := spiderpoolfake.NewSimpleClientset(&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "spider-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:      &cniType,
					IPVlanConfig: &v2beta1.SpiderIPvlanCniConfig{Master: []string{"eth1"}},
				},
			})
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-a",
					Namespace: "tenant-a",
					Annotations: map[string]string{
						constant.MultusNetworkAttachmentAnnot: "tenant-a/external-net,tenant-a/spider-net",
					},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			}

			err := podMasterNICResourceMutatingWebhook(ctx, spiderClient, pod, PodENIResourceInjectConfig{
				MasterNICEnabled:      true,
				InjectPodENIResources: true,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers[0].Resources.Limits[resourceName]).To(Equal(resource.MustParse("1")))
		})

		It("should skip injection when master NIC advertisement is disabled", func() {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Namespace:   "tenant-a",
					Annotations: map[string]string{constant.MultusNetworkAttachmentAnnot: "tenant-a/net-a"},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			}

			err := podMasterNICResourceMutatingWebhook(context.Background(), spiderpoolfake.NewSimpleClientset(), pod, PodENIResourceInjectConfig{
				MasterNICEnabled:      false,
				InjectPodENIResources: true,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers[0].Resources.Limits).To(BeEmpty())
		})

		It("should inject master NIC resources even when provider mode is enabled", func() {
			// FR-033 / SC-013: master NIC scheduling is independent of provider
			// mode, so injection must happen with ProviderEnabled=true as long as
			// MasterNICEnabled and InjectPodENIResources are true.
			ctx := context.Background()
			cniType := constant.MacvlanCNI
			resourceName := corev1.ResourceName("spidernet.io/eth1-nic")
			spiderClient := spiderpoolfake.NewSimpleClientset(&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "macvlan-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:       &cniType,
					MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{Master: []string{"eth1"}},
				},
			})
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "pod-a",
					Namespace:   "tenant-a",
					Annotations: map[string]string{constant.MultusNetworkAttachmentAnnot: "tenant-a/macvlan-net"},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}},
			}

			err := podMasterNICResourceMutatingWebhook(ctx, spiderClient, pod, PodENIResourceInjectConfig{
				ProviderEnabled:       true,
				MasterNICEnabled:      true,
				InjectPodENIResources: true,
			})

			Expect(err).NotTo(HaveOccurred())
			Expect(pod.Spec.Containers[0].Resources.Limits[resourceName]).To(Equal(resource.MustParse("1")))
			Expect(pod.Spec.Containers[0].Resources.Requests[resourceName]).To(Equal(resource.MustParse("1")))
		})
	})

	Describe("needPodNetworkInjection", func() {
		It("should return false when neither Pod nor Namespace defines the annotation", func() {
			ctx := context.Background()
			nsManager := &stubNamespaceManager{
				namespace: &corev1.Namespace{
					ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"},
				},
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "pod-a",
					Namespace: "tenant-a",
				},
			}

			needInject, err := needPodNetworkInjection(ctx, nsManager, pod)
			Expect(err).NotTo(HaveOccurred())
			Expect(needInject).To(BeFalse())
		})
	})

	Describe("podNetworkMutatingWebhook", func() {
		var (
			ctx       context.Context
			nsManager *stubNamespaceManager
		)

		BeforeEach(func() {
			ctx = context.Background()
		})

		Context("when Pod has no annotations and Namespace provides the injection value", func() {
			BeforeEach(func() {
				nsManager = &stubNamespaceManager{
					namespace: &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "tenant-a",
							Annotations: map[string]string{
								constant.AnnoNetworkResourceInject: "vlan100",
							},
						},
					},
				}
			})

			It("should inject Multus annotation and RDMA resource using Namespace fallback", func() {
				cniType := constant.SriovCNI
				resourceName := "rdma/shared_a"
				spiderClient := spiderpoolfake.NewSimpleClientset(&v2beta1.SpiderMultusConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "spiderpool.spidernet.io/v2beta1",
						Kind:       constant.KindSpiderMultusConfig,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vlan100-net",
						Namespace: "spiderpool",
						Labels: map[string]string{
							constant.AnnoNetworkResourceInject: "vlan100",
						},
					},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: &cniType,
						SriovConfig: &v2beta1.SpiderSRIOVCniConfig{
							ResourceName: &resourceName,
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"pool-a"},
							},
						},
					},
				})
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-a",
						Namespace: "tenant-a",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "app"}},
					},
				}

				Expect(podNetworkMutatingWebhook(ctx, spiderClient, nsManager, pod)).To(Succeed())
				Expect(pod.Annotations).NotTo(BeNil())
				Expect(pod.Annotations[constant.MultusNetworkAttachmentAnnot]).To(Equal("spiderpool/vlan100-net"))
				_, hasRDMA := pod.Spec.Containers[0].Resources.Limits[corev1.ResourceName(resourceName)]
				Expect(hasRDMA).To(BeTrue(), "expected RDMA resource %q in pod limits", resourceName)
				Expect(nsManager.callCount).To(BeNumerically(">", 0), "namespace manager should be consulted")
			})
		})

		Context("when Pod defines AnnoNetworkResourceInject and Namespace defines the same annotation", func() {
			BeforeEach(func() {
				nsManager = &stubNamespaceManager{
					namespace: &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name: "tenant-a",
							Annotations: map[string]string{
								constant.AnnoNetworkResourceInject: "vlan100",
							},
						},
					},
				}
			})

			It("should prefer the Pod annotation and not consult the Namespace", func() {
				cniType := constant.SriovCNI
				resourceName := "rdma/shared_b"
				spiderClient := spiderpoolfake.NewSimpleClientset(&v2beta1.SpiderMultusConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "spiderpool.spidernet.io/v2beta1",
						Kind:       constant.KindSpiderMultusConfig,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vlan200-net",
						Namespace: "spiderpool",
						Labels: map[string]string{
							constant.AnnoNetworkResourceInject: "vlan200",
						},
					},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: &cniType,
						SriovConfig: &v2beta1.SpiderSRIOVCniConfig{
							ResourceName: &resourceName,
							SpiderpoolConfigPools: &v2beta1.SpiderpoolPools{
								IPv4IPPool: []string{"pool-b"},
							},
						},
					},
				})
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-b",
						Namespace: "tenant-a",
						Annotations: map[string]string{
							constant.AnnoNetworkResourceInject: "vlan200",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "app"}},
					},
				}

				Expect(podNetworkMutatingWebhook(ctx, spiderClient, nsManager, pod)).To(Succeed())
				Expect(pod.Annotations[constant.MultusNetworkAttachmentAnnot]).To(Equal("spiderpool/vlan200-net"))
				// AnnoPodResourceInject is checked first; Pod lacks it so nsManager is consulted once for
				// that annotation. AnnoNetworkResourceInject is found on the Pod directly (no ns lookup).
				Expect(nsManager.callCount).To(Equal(1), "namespace manager should be called once for AnnoPodResourceInject fallback")
			})
		})

		Context("when neither Pod nor Namespace defines the annotation", func() {
			BeforeEach(func() {
				nsManager = &stubNamespaceManager{
					namespace: &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"},
					},
				}
			})

			It("should be a no-op and not inject any Multus annotation", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-c",
						Namespace: "tenant-a",
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "app"}},
					},
				}

				Expect(podNetworkMutatingWebhook(ctx, spiderpoolfake.NewSimpleClientset(), nsManager, pod)).To(Succeed())
				if pod.Annotations != nil {
					Expect(pod.Annotations).NotTo(HaveKey(constant.MultusNetworkAttachmentAnnot))
				}
			})
		})

		Context("when Pod uses network resource injection with vlan auto mode", func() {
			BeforeEach(func() {
				nsManager = &stubNamespaceManager{
					namespace: &corev1.Namespace{
						ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"},
					},
				}
			})

			It("should inject the Multus annotation without requiring vlanID or RDMA resource", func() {
				spiderClient := spiderpoolfake.NewSimpleClientset(&v2beta1.SpiderMultusConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "spiderpool.spidernet.io/v2beta1",
						Kind:       constant.KindSpiderMultusConfig,
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "vlan-auto-net",
						Namespace: "spiderpool",
						Labels: map[string]string{
							constant.AnnoNetworkResourceInject: "provider-vlan",
						},
					},
					Spec: v2beta1.MultusCNIConfigSpec{
						CniType: ptr.To(constant.VlanCNI),
						VlanConfig: &v2beta1.SpiderVlanCniConfig{
							Master:   []string{"eth0"},
							VlanMode: ptr.To(constant.VlanModeAuto),
						},
					},
				})
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pod-vlan-auto",
						Namespace: "tenant-a",
						Annotations: map[string]string{
							constant.AnnoNetworkResourceInject: "provider-vlan",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{Name: "app"}},
					},
				}

				Expect(podNetworkMutatingWebhook(ctx, spiderClient, nsManager, pod)).To(Succeed())
				Expect(pod.Annotations[constant.MultusNetworkAttachmentAnnot]).To(Equal("spiderpool/vlan-auto-net"))
				Expect(pod.Spec.Containers[0].Resources.Limits).To(BeEmpty())
			})
		})
	})
})
