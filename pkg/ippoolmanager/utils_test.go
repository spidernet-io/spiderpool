// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
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

})
