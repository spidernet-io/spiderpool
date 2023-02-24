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

var _ = Describe("JobInformer", Label("unitest"), func() {

	Context("UT job_informer", Serial, func() {
		defer GinkgoRecover()
		job1 := batchv1.Job{}
		job2 := batchv1.Job{}
		reconcile := func(ctx context.Context, oldObj, newObj interface{}) error {
			return constant.ErrUnknown
		}

		cleanup := func(ctx context.Context, obj interface{}) error {
			return constant.ErrUnknown
		}
		logger := logutils.Logger.Named("ut-test-job-informer")

		// NewApplicationController
		controller, err := NewApplicationController(reconcile, cleanup, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onJobAdd", func() {
			controller.onJobAdd(&job1)
		})
		It("failed to onJobUpdate", func() {
			controller.onJobUpdate(&job1, &job2)
		})
		It("failed to onJobDelete", func() {
			controller.onJobDelete(&job1)
		})
		It("AddJobController", func() {
			jobInformer := factory.Batch().V1().Jobs().Informer()
			controller.AddJobController(jobInformer)
		})
	})
})
