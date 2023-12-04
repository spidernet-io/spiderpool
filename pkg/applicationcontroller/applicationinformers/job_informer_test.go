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

var _ = Describe("JobInformer", Label("unittest"), func() {
	Context("UT job_informer", Serial, func() {
		job1 := &batchv1.Job{}
		job2 := &batchv1.Job{}

		logger := logutils.Logger.Named("ut-test-job-informer")

		// NewApplicationController
		controller, err := NewApplicationController(fakeReconcileFunc, fakeCleanupFunc, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onJobAdd", func() {
			controller.onJobAdd(job1)
		})

		It("failed to onJobUpdate", func() {
			controller.onJobUpdate(job1, job2)
		})

		It("failed to onJobDelete", func() {
			controller.onJobDelete(job1)
		})

		It("AddJobController successfully", func() {
			jobInformer := factory.Batch().V1().Jobs().Informer()

			err := controller.AddJobController(jobInformer)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fail to AddCronJobHandler", func() {
			jobInformer := factory.Batch().V1().Jobs().Informer()
			patch := gomonkey.ApplyMethodReturn(jobInformer, "AddEventHandler", nil, constant.ErrUnknown)
			defer patch.Reset()

			err := controller.AddJobController(jobInformer)
			Expect(err).To(HaveOccurred())
		})
	})
})
