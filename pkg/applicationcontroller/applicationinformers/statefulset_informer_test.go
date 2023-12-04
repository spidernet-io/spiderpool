// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appv1 "k8s.io/api/apps/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var _ = Describe("StatefulSetInformer", Label("unittest"), func() {
	Context("UT statefulSet_informer", Serial, func() {
		ds1 := &appv1.StatefulSet{}
		ds2 := &appv1.StatefulSet{}

		logger := logutils.Logger.Named("ut-test-statefulSet-informer")

		// NewApplicationController
		controller, err := NewApplicationController(fakeReconcileFunc, fakeCleanupFunc, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onStatefulSetAdd", func() {
			controller.onStatefulSetAdd(ds1)
		})

		It("failed to onStatefulSetUpdate", func() {
			controller.onStatefulSetUpdate(ds1, ds2)
		})

		It("failed to onStatefulSetDelete", func() {
			controller.onStatefulSetDelete(ds1)
		})

		It("AddStatefulSetHandler", func() {
			statefulSetInformer := factory.Apps().V1().StatefulSets().Informer()

			err := controller.AddStatefulSetHandler(statefulSetInformer)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fail to AddCronJobHandler", func() {
			statefulSetInformer := factory.Apps().V1().StatefulSets().Informer()
			patch := gomonkey.ApplyMethodReturn(statefulSetInformer, "AddEventHandler", nil, constant.ErrUnknown)
			defer patch.Reset()

			err := controller.AddStatefulSetHandler(statefulSetInformer)
			Expect(err).To(HaveOccurred())
		})
	})
})
