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

var _ = Describe("DeploymentInformer", Label("unitest"), func() {

	Context("UT deployment_informer", Serial, func() {
		defer GinkgoRecover()

		deploy1 := appv1.Deployment{}
		deploy2 := appv1.Deployment{}

		reconcile := func(ctx context.Context, oldObj, newObj interface{}) error {
			return constant.ErrUnknown
		}

		cleanup := func(ctx context.Context, obj interface{}) error {
			return constant.ErrUnknown
		}

		logger := logutils.Logger.Named("ut-test-deploy-informer")

		// NewApplicationController
		controller, err := NewApplicationController(reconcile, cleanup, logger)
		Expect(err).NotTo(HaveOccurred())

		It("failed to onDeploymentAdd", func() {
			controller.onDeploymentAdd(&deploy1)
		})
		It("failed to onDeploymentUpdate", func() {
			controller.onDeploymentUpdate(&deploy1, &deploy2)
		})
		It("failed to onDeploymentDelete", func() {
			controller.onDeploymentDelete(&deploy1)
		})
		It("AddDeploymentHandler", func() {
			deploymentInformer := factory.Apps().V1().Deployments().Informer()
			controller.AddDaemonSetHandler(deploymentInformer)
		})
	})
})
