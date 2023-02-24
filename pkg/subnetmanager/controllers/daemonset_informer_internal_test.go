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

var _ = Describe("DaemonSetInformer", Label("unitest"), func() {

	Context("UT daemonSet_informer", Serial, func() {
		defer GinkgoRecover()

		ds1 := appv1.DaemonSet{}
		ds2 := appv1.DaemonSet{}

		reconcile := func(ctx context.Context, oldObj, newObj interface{}) error {
			return constant.ErrUnknown
		}

		cleanup := func(ctx context.Context, obj interface{}) error {
			return constant.ErrUnknown
		}

		logger := logutils.Logger.Named("ut-test-daemonSet-informer")

		// NewApplicationController
		controller, err := NewApplicationController(reconcile, cleanup, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onDaemonSetAdd", func() {
			controller.onDaemonSetAdd(&ds1)
		})
		It("failed to onDaemonSetUpdate", func() {
			controller.onDaemonSetUpdate(&ds1, &ds2)
		})
		It("failed to onDaemonSetDelete", func() {
			controller.onDaemonSetDelete(&ds1)
		})
		It("AddDaemonSetHandler", func() {
			daemonSetInformer := factory.Apps().V1().DaemonSets().Informer()
			controller.AddDaemonSetHandler(daemonSetInformer)
		})
	})
})
