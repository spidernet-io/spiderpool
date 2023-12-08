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

var _ = Describe("DaemonSetInformer", Label("unittest"), func() {
	Context("UT daemonSet_informer", Serial, func() {
		ds1 := &appv1.DaemonSet{}
		ds2 := &appv1.DaemonSet{}

		logger := logutils.Logger.Named("ut-test-daemonSet-informer")

		// NewApplicationController
		controller, err := NewApplicationController(fakeReconcileFunc, fakeCleanupFunc, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onDaemonSetAdd", func() {
			controller.onDaemonSetAdd(ds1)
		})

		It("failed to onDaemonSetUpdate", func() {
			controller.onDaemonSetUpdate(ds1, ds2)
		})

		It("failed to onDaemonSetDelete", func() {
			controller.onDaemonSetDelete(ds1)
		})

		It("AddDaemonSetHandler successfully", func() {
			daemonSetInformer := factory.Apps().V1().DaemonSets().Informer()

			err := controller.AddDaemonSetHandler(daemonSetInformer)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fail to AddDaemonSetHandler", func() {
			daemonSetInformer := factory.Apps().V1().DaemonSets().Informer()
			patch := gomonkey.ApplyMethodReturn(daemonSetInformer, "AddEventHandler", nil, constant.ErrUnknown)
			defer patch.Reset()

			err := controller.AddDaemonSetHandler(daemonSetInformer)
			Expect(err).To(HaveOccurred())
		})
	})
})
