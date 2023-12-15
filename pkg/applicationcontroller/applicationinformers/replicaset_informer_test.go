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

var _ = Describe("ReplicaSetInformer", Label("unittest"), func() {
	Context("UT ReplicaSet_informer", Serial, func() {
		ds1 := &appv1.ReplicaSet{}
		ds2 := &appv1.ReplicaSet{}

		logger := logutils.Logger.Named("ut-test-replicaSet-informer")

		// NewApplicationController
		controller, err := NewApplicationController(fakeReconcileFunc, fakeCleanupFunc, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onReplicaSetAdd", func() {
			controller.onReplicaSetAdd(ds1)
		})

		It("failed to onReplicaSetUpdate", func() {
			controller.onReplicaSetUpdate(ds1, ds2)
		})

		It("failed to onReplicaSetDelete", func() {
			controller.onReplicaSetDelete(ds1)
		})

		It("AddReplicaSetHandler successfully", func() {
			replicaSetsInformer := factory.Apps().V1().ReplicaSets().Informer()

			err := controller.AddReplicaSetHandler(replicaSetsInformer)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fail to AddCronJobHandler", func() {
			replicaSetsInformer := factory.Apps().V1().ReplicaSets().Informer()
			patch := gomonkey.ApplyMethodReturn(replicaSetsInformer, "AddEventHandler", nil, constant.ErrUnknown)
			defer patch.Reset()

			err := controller.AddReplicaSetHandler(replicaSetsInformer)
			Expect(err).To(HaveOccurred())
		})
	})
})
