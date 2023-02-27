// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	appv1 "k8s.io/api/apps/v1"
)

var _ = Describe("StatefulSetInformer", Label("unitest"), func() {

	Context("UT statefulSet_informer", Serial, func() {
		defer GinkgoRecover()

		ds1 := appv1.StatefulSet{}
		ds2 := appv1.StatefulSet{}

		reconcile := func(ctx context.Context, oldObj, newObj interface{}) error {
			return constant.ErrUnknown
		}

		cleanup := func(ctx context.Context, obj interface{}) error {
			return constant.ErrUnknown
		}

		logger := logutils.Logger.Named("ut-test-statefulSet-informer")

		// NewApplicationController
		controller, err := NewApplicationController(reconcile, cleanup, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onStatefulSetAdd", func() {
			controller.onStatefulSetAdd(&ds1)
		})
		It("failed to onStatefulSetUpdate", func() {
			controller.onStatefulSetUpdate(&ds1, &ds2)
		})
		It("failed to onStatefulSetDelete", func() {
			controller.onStatefulSetDelete(&ds1)
		})
		It("AddStatefulSetHandler", func() {
			statefulSetInformer := factory.Apps().V1().StatefulSets().Informer()
			controller.AddDaemonSetHandler(statefulSetInformer)
		})
	})
})
