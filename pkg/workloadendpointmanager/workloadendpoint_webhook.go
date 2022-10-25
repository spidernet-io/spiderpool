// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var webhookLogger *zap.Logger

func (wm *workloadEndpointManager) SetupWebhook() error {
	if webhookLogger == nil {
		webhookLogger = logutils.Logger.Named("Endpoint-Webhook")
	}

	return ctrl.NewWebhookManagedBy(wm.runtimeMgr).
		For(&spiderpoolv1.SpiderEndpoint{}).
		WithDefaulter(wm).
		Complete()
}

var _ webhook.CustomDefaulter = (*workloadEndpointManager)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (rm *workloadEndpointManager) Default(ctx context.Context, obj runtime.Object) error {
	endpoint, ok := obj.(*spiderpoolv1.SpiderEndpoint)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("mutating webhook of Endpoint got an object with mismatched GVK: %+v", obj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Mutating").With(
		zap.String("EndpointName", endpoint.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Info("Start to mutate Endpoint")
	logger.Sugar().Debugf("Request Endpoint: %+v", *endpoint)

	if endpoint.DeletionTimestamp != nil {
		logger.Info("Deleting Endpoint, noting to mutate")
		return nil
	}

	if !controllerutil.ContainsFinalizer(endpoint, constant.SpiderFinalizer) {
		controllerutil.AddFinalizer(endpoint, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer %s", constant.SpiderFinalizer)
	}

	return nil
}
