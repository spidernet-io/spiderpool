// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var _ = Describe("CronjobInformer", Label("unittest"), func() {
	Context("UT cronjob_informer", func() {
		cronObj1 := &batchv1.CronJob{}
		cronObj2 := &batchv1.CronJob{}

		logger := logutils.Logger.Named("ut-test-cronjob-informer")

		// NewApplicationController
		controller, err := NewApplicationController(fakeReconcileFunc, fakeCleanupFunc, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onCronJobAdd", func() {
			controller.onCronJobAdd(cronObj1)
		})

		It("failed to onCronJobUpdate", func() {
			controller.onCronJobUpdate(cronObj1, cronObj2)
		})

		It("failed to onCronJobDelete", func() {
			controller.onCronJobDelete(cronObj1)
		})

		It("AddCronJobHandler successfully", func() {
			cronJobInformer := factory.Batch().V1().CronJobs().Informer()

			err := controller.AddCronJobHandler(cronJobInformer)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fail to AddCronJobHandler", func() {
			cronJobInformer := factory.Batch().V1().CronJobs().Informer()
			patch := gomonkey.ApplyMethodReturn(cronJobInformer, "AddEventHandler", nil, constant.ErrUnknown)
			defer patch.Reset()

			err := controller.AddCronJobHandler(cronJobInformer)
			Expect(err).To(HaveOccurred())
		})
	})
})
