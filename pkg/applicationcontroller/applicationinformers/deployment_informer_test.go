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

var _ = Describe("DeploymentInformer", Label("unittest"), func() {
	Context("UT deployment_informer", Serial, func() {
		deploy1 := &appv1.Deployment{}
		deploy2 := &appv1.Deployment{}

		logger := logutils.Logger.Named("ut-test-deploy-informer")

		// NewApplicationController
		controller, err := NewApplicationController(fakeReconcileFunc, fakeCleanupFunc, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onDeploymentAdd", func() {
			controller.onDeploymentAdd(deploy1)
		})

		It("failed to onDeploymentUpdate", func() {
			controller.onDeploymentUpdate(deploy1, deploy2)
		})

		It("failed to onDeploymentDelete", func() {
			controller.onDeploymentDelete(deploy1)
		})

		It("AddDeploymentHandler successfully", func() {
			deploymentInformer := factory.Apps().V1().Deployments().Informer()

			err := controller.AddDeploymentHandler(deploymentInformer)
			Expect(err).NotTo(HaveOccurred())
		})

		It("fail to AddCronJobHandler", func() {
			deploymentInformer := factory.Apps().V1().Deployments().Informer()
			patch := gomonkey.ApplyMethodReturn(deploymentInformer, "AddEventHandler", nil, constant.ErrUnknown)
			defer patch.Reset()

			err := controller.AddDeploymentHandler(deploymentInformer)
			Expect(err).To(HaveOccurred())
		})
	})
})
