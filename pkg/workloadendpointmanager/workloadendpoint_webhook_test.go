// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"context"
	"fmt"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var _ = Describe("WorkloadEndpointWebhook", Label("workloadendpoint_webhook_test"), func() {
	Describe("Set up WorkloadEndpointWebhook", func() {
		PIt("talks to a Kubernetes API server", func() {
			cfg, err := config.GetConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).NotTo(BeNil())

			mgr, err := ctrl.NewManager(cfg, manager.Options{Scheme: scheme})
			Expect(err).NotTo(HaveOccurred())

			err = endpointWebhook.SetupWebhookWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Test WorkloadEndpointWebhook's method", func() {
		var endpointT *spiderpoolv1.SpiderEndpoint

		BeforeEach(func() {
			workloadendpointmanager.WebhookLogger = logutils.Logger.Named("Endpoint-Webhook")
			endpointT = &spiderpoolv1.SpiderEndpoint{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderEndpointKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "endpoint",
					Namespace: "default",
				},
			}
		})

		Describe("Default", func() {
			It("avoids modifying the terminating Endpoint", func() {
				deletionTimestamp := metav1.NewTime(time.Now().Add(30 * time.Second))
				endpointT.SetDeletionTimestamp(&deletionTimestamp)

				ctx := context.TODO()
				err := endpointWebhook.Default(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds finalizer", func() {
				ctx := context.TODO()
				err := endpointWebhook.Default(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				contains := controllerutil.ContainsFinalizer(endpointT, constant.SpiderFinalizer)
				Expect(contains).To(BeTrue())
			})

			It("failed to mutate Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyPrivateMethod(
					endpointWebhook,
					"mutateWorkloadEndpoint",
					func(_ context.Context, _ *spiderpoolv1.SpiderEndpoint) error {
						return constant.ErrUnknown
					},
				)
				defer patches.Reset()

				ctx := context.TODO()
				err := endpointWebhook.Default(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
