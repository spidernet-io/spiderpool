// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

var _ = Describe("Controller", Label("unitest"), func() {
	Context("NewApplicationController", func() {
		type controllerAgs struct {
			reconcileFunc AppInformersAddOrUpdateFunc
			cleanupFunc   APPInformersDelFunc
			logger        *zap.Logger

			expect bool
		}
		var conrollerags controllerAgs
		BeforeEach(func() {
			conrollerags = controllerAgs{
				reconcileFunc: func(ctx context.Context, oldObj, newObj interface{}) error {
					return nil
				},
				cleanupFunc: func(ctx context.Context, obj interface{}) error {
					return nil
				},
				logger: &zap.Logger{},
			}
		})

		DescribeTable("UT NewApplicationController", Label("NewApplicationController"), func(getParam func() *controllerAgs) {
			p := getParam()
			if p.reconcileFunc == nil || p.cleanupFunc == nil {
				_, err := NewApplicationController(p.reconcileFunc, p.cleanupFunc, p.logger)
				Expect(err).To(HaveOccurred())
			}
			if p.expect {
				_, err := NewApplicationController(p.reconcileFunc, p.cleanupFunc, p.logger)
				Expect(err).NotTo(HaveOccurred())
			}
		},
			Entry("nil reconcile", func() *controllerAgs {
				conrollerags.reconcileFunc = nil
				return &conrollerags
			}),
			Entry("nil cleanup", func() *controllerAgs {
				conrollerags.cleanupFunc = nil
				return &conrollerags
			}),
		)
	})
})
