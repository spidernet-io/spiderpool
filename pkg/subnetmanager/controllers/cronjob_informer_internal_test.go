// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	batchv1 "k8s.io/api/batch/v1"
)

var _ = Describe("CronjobInformer", Label("unitest"), func() {

	Context("UT cronjob_informer", Serial, func() {
		defer GinkgoRecover()
		cronObj1 := batchv1.CronJob{}
		cronObj2 := batchv1.CronJob{}
		reconcile := func(ctx context.Context, oldObj, newObj interface{}) error {
			return constant.ErrUnknown
		}

		cleanup := func(ctx context.Context, obj interface{}) error {
			return constant.ErrUnknown
		}
		logger := logutils.Logger.Named("ut-test-cronjob-informer")

		// NewApplicationController
		controller, err := NewApplicationController(reconcile, cleanup, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onCronJobAdd", func() {
			controller.onCronJobAdd(&cronObj1)
		})
		It("failed to onCronJobUpdate", func() {
			controller.onCronJobUpdate(&cronObj1, &cronObj2)
		})
		It("failed to onCronJobDelete", func() {
			controller.onCronJobDelete(&cronObj1)
		})
		It("AddCronJobHandler", func() {
			cronJobInformer := factory.Batch().V1().CronJobs().Informer()
			controller.AddJobController(cronJobInformer)
		})
	})
})
