// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var _ = Describe("Controller", Label("unittest"), func() {
	Context("NewApplicationController", func() {
		log := logutils.Logger.Named("application-informers-ut")

		It("fail to new application controller with no reconcile function", func() {
			_, err := NewApplicationController(nil, fakeCleanupFunc, log)
			Expect(err).To(HaveOccurred())
		})

		It("fail to new application controller with no cleanup function", func() {
			_, err := NewApplicationController(fakeReconcileFunc, nil, log)
			Expect(err).To(HaveOccurred())
		})

		It("new application controller successfully", func() {
			controller, err := NewApplicationController(fakeReconcileFunc, fakeCleanupFunc, log)
			Expect(err).NotTo(HaveOccurred())
			Expect(controller).NotTo(BeNil())
		})
	})
})
