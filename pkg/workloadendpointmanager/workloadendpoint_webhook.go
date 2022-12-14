// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var WebhookLogger *zap.Logger

type WorkloadEndpointWebhook struct {
}

func (ew *WorkloadEndpointWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if WebhookLogger == nil {
		WebhookLogger = logutils.Logger.Named("Endpoint-Webhook")
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv1.SpiderEndpoint{}).
		WithDefaulter(ew).
		Complete()
}

var _ webhook.CustomDefaulter = (*WorkloadEndpointWebhook)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (ew *WorkloadEndpointWebhook) Default(ctx context.Context, obj runtime.Object) error {
	endpoint := obj.(*spiderpoolv1.SpiderEndpoint)

	logger := WebhookLogger.Named("Mutating").With(
		zap.String("EndpointName", endpoint.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Sugar().Debugf("Request Endpoint: %+v", *endpoint)

	if err := ew.mutateWorkloadEndpoint(logutils.IntoContext(ctx, logger), endpoint); err != nil {
		logger.Sugar().Errorf("Failed to mutate Endpoint: %v", err)
	}

	return nil
}
