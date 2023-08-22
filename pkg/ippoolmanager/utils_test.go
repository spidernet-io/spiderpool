// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types2 "k8s.io/apimachinery/pkg/types"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var _ = Describe("IPPoolManager-utils", Label("ippool_manager_utils"), func() {
	Context("IsAutoCreatedIPPool", Labels{"unitest", "IsAutoCreatedIPPool"}, func() {
		It("normal IPPool", func() {
			var pool spiderpoolv2beta1.SpiderIPPool

			label := map[string]string{constant.LabelIPPoolOwnerApplicationName: "test-name"}
			pool.SetLabels(label)

			isAutoCreatedIPPool := IsAutoCreatedIPPool(&pool)
			Expect(isAutoCreatedIPPool).To(BeTrue())
		})

		It("auto-created IPPool", func() {
			var pool spiderpoolv2beta1.SpiderIPPool

			isAutoCreatedIPPool := IsAutoCreatedIPPool(&pool)
			Expect(isAutoCreatedIPPool).To(BeFalse())
		})
	})

	Context("Test Auto IPPool PodAffinity", Labels{"unitest", "AutoPool-PodAffinity"}, func() {
		It("match auto-created IPPool affinity", func() {
			podTopController := types.PodTopController{
				AppNamespacedName: types.AppNamespacedName{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       constant.KindDeployment,
					Namespace:  "test-ns",
					Name:       "test-name",
				},
				UID: types2.UID("a-b-c"),
				APP: nil,
			}

			podAffinity := NewAutoPoolPodAffinity(podTopController)

			isMatchAutoPoolAffinity := IsMatchAutoPoolAffinity(podAffinity, podTopController)
			Expect(isMatchAutoPoolAffinity).To(BeTrue())
		})

		It("not match auto-created IPPool affinity", func() {
			podTopController := types.PodTopController{
				AppNamespacedName: types.AppNamespacedName{
					APIVersion: appsv1.SchemeGroupVersion.String(),
					Kind:       constant.KindDeployment,
					Namespace:  "test-ns",
					Name:       "test-name",
				},
				UID: types2.UID("a-b-c"),
				APP: nil,
			}

			podAffinity := NewAutoPoolPodAffinity(podTopController)

			podTopController.Kind = constant.KindStatefulSet
			isMatchAutoPoolAffinity := IsMatchAutoPoolAffinity(podAffinity, podTopController)
			Expect(isMatchAutoPoolAffinity).To(BeFalse())
		})
	})

	Context("Test IPAM pool candidates order selections", func() {
		poolTemplate := &spiderpoolv2beta1.SpiderIPPool{}

		Context("IPPool with PodAffinity", func() {
			var pool1, pool2 *spiderpoolv2beta1.SpiderIPPool
			BeforeEach(func() {
				pool1 = poolTemplate.DeepCopy()
				pool1.SetName("pool1")
				pool1.Spec.PodAffinity = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"PodAffinityKey": "PodAffinityValue1",
					},
				}

				pool2 = poolTemplate.DeepCopy()
				pool2.SetName("pool2")
			})

			It("preorder", func() {
				byPoolPriority := ByPoolPriority{pool1, pool2}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})

			It("postorder", func() {
				byPoolPriority := ByPoolPriority{pool2, pool1}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})
		})

		Context("IPPool with NodeAffinity", func() {
			var pool1, pool2 *spiderpoolv2beta1.SpiderIPPool
			BeforeEach(func() {
				pool1 = poolTemplate.DeepCopy()
				pool1.SetName("pool1")
				pool1.Spec.NodeAffinity = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"NodeAffinityKey": "NodeAffinityValue1",
					},
				}

				pool2 = poolTemplate.DeepCopy()
				pool2.SetName("pool2")
			})

			It("preorder", func() {
				byPoolPriority := ByPoolPriority{pool1, pool2}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})

			It("postorder", func() {
				byPoolPriority := ByPoolPriority{pool2, pool1}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})
		})

		Context("IPPool with NamespaceAffinity", func() {
			var pool1, pool2 *spiderpoolv2beta1.SpiderIPPool
			BeforeEach(func() {
				pool1 = poolTemplate.DeepCopy()
				pool1.SetName("pool1")
				pool1.Spec.NamespaceAffinity = &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"NamespaceAffinityKey": "NamespaceAffinityValue1",
					},
				}

				pool2 = poolTemplate.DeepCopy()
				pool2.SetName("pool2")
			})

			It("preorder", func() {
				byPoolPriority := ByPoolPriority{pool1, pool2}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})

			It("postorder", func() {
				byPoolPriority := ByPoolPriority{pool2, pool1}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})
		})

		Context("IPPool with MultusName", func() {
			var pool1, pool2 *spiderpoolv2beta1.SpiderIPPool
			BeforeEach(func() {
				pool1 = poolTemplate.DeepCopy()
				pool1.SetName("pool1")
				pool1.Spec.MultusName = []string{"kube-system/macvlan"}

				pool2 = poolTemplate.DeepCopy()
				pool2.SetName("pool2")
			})

			It("preorder", func() {
				byPoolPriority := ByPoolPriority{pool1, pool2}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})

			It("postorder", func() {
				byPoolPriority := ByPoolPriority{pool2, pool1}
				sort.Sort(byPoolPriority)
				Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
			})
		})

		It("no any affinities", func() {
			pool1 := poolTemplate.DeepCopy()
			pool1.SetName("pool1")
			pool2 := poolTemplate.DeepCopy()
			pool2.SetName("pool2")

			byPoolPriority := ByPoolPriority{pool1, pool2}
			sort.Sort(byPoolPriority)
			Expect(byPoolPriority).To(Equal(ByPoolPriority{pool1, pool2}))
		})

		It("pool affinity priority in sequence", func() {
			pool1 := poolTemplate.DeepCopy()
			pool1.SetName("pool1")
			pool1.Spec.PodAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"PodAffinityKey": "PodAffinityValue1",
				},
			}

			pool2 := poolTemplate.DeepCopy()
			pool2.SetName("pool2")
			pool2.Spec.NodeName = []string{"master"}
			pool2.Spec.NodeAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"NodeAffinityKey1": "NodeAffinityValue1",
				},
			}

			pool3 := poolTemplate.DeepCopy()
			pool3.SetName("pool3")
			pool3.Spec.NamespaceName = []string{"kube-system"}
			pool3.Spec.NamespaceAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"NamespaceAffinityKey1": "NamespaceAffinityValue1",
				},
			}

			pool4 := poolTemplate.DeepCopy()
			pool4.SetName("pool4")
			pool4.Spec.MultusName = []string{"kube-system/macvlan"}

			pool5 := poolTemplate.DeepCopy()
			pool5.SetName("pool5")

			byPoolPriority := ByPoolPriority{pool1, pool2, pool3, pool4, pool5}
			sort.Sort(byPoolPriority)
			Expect(byPoolPriority).Should(Equal(ByPoolPriority{pool1, pool2, pool3, pool4, pool5}))
		})

		It("pool affinity priority in multiple cases", func() {
			pool1 := poolTemplate.DeepCopy()
			pool1.SetName("pool1")
			pool1.Spec.PodAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"PodAffinityKey": "PodAffinityValue1",
				},
			}

			pool2 := poolTemplate.DeepCopy()
			pool2.SetName("pool2")
			pool2.Spec.NodeName = []string{"master"}

			pool3 := poolTemplate.DeepCopy()
			pool3.SetName("pool3")
			pool3.Spec.NodeName = []string{"master"}
			pool3.Spec.NamespaceName = []string{"kube-system"}

			pool4 := poolTemplate.DeepCopy()
			pool4.SetName("pool4")
			pool4.Spec.NamespaceName = []string{"kube-system"}
			pool4.Spec.MultusName = []string{"kube-system/macvlan"}

			byPoolPriority := ByPoolPriority{pool1, pool2, pool3, pool4}
			sort.Sort(byPoolPriority)
			Expect(byPoolPriority).Should(Equal(ByPoolPriority{pool1, pool3, pool2, pool4}))
		})

		It("pool affinity priority in chaos cases", func() {
			pool1 := poolTemplate.DeepCopy()
			pool1.SetName("pool1")
			pool1.Spec.PodAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"PodAffinityKey": "PodAffinityValue1",
				},
			}
			pool1.Spec.NamespaceName = []string{"kube-system"}
			pool1.Spec.MultusName = []string{"kube-system/macvlan"}

			pool2 := poolTemplate.DeepCopy()
			pool2.SetName("pool2")
			pool2.Spec.PodAffinity = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"PodAffinityKey": "PodAffinityValue1",
				},
			}
			pool2.Spec.NodeName = []string{"master"}

			byPoolPriority := ByPoolPriority{pool1, pool2}
			sort.Sort(byPoolPriority)
			Expect(byPoolPriority).Should(Equal(ByPoolPriority{pool2, pool1}))
		})
	})
})
