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

var _ = Describe("ReplicaSetInformer", Label("unitest"), func() {

	Context("UT ReplicaSet_informer", Serial, func() {
		defer GinkgoRecover()

		ds1 := appv1.ReplicaSet{}
		ds2 := appv1.ReplicaSet{}

		reconcile := func(ctx context.Context, oldObj, newObj interface{}) error {
			return constant.ErrUnknown
		}

		cleanup := func(ctx context.Context, obj interface{}) error {
			return constant.ErrUnknown
		}

		logger := logutils.Logger.Named("ut-test-replicaSet-informer")

		// NewApplicationController
		controller, err := NewApplicationController(reconcile, cleanup, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onReplicaSetAdd", func() {
			controller.onReplicaSetAdd(&ds1)
		})
		It("failed to onReplicaSetUpdate", func() {
			controller.onReplicaSetUpdate(&ds1, &ds2)
		})
		It("failed to onReplicaSetDelete", func() {
			controller.onReplicaSetDelete(&ds1)
		})
		It("AddReplicaSetHandler", func() {
			replicaSetsInformer := factory.Apps().V1().ReplicaSets().Informer()
			controller.AddDaemonSetHandler(replicaSetsInformer)
		})
	})
})
