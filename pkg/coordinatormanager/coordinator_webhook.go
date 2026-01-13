// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"context"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var WebhookLogger *zap.Logger

type CoordinatorWebhook struct{}

func (cw *CoordinatorWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if WebhookLogger == nil {
		WebhookLogger = logutils.Logger.Named("Coordinator-Webhook")
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv2beta1.SpiderCoordinator{}).
		WithDefaulter(cw).
		WithValidator(cw).
		Complete()
}

var _ webhook.CustomDefaulter = (*CoordinatorWebhook)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (cw *CoordinatorWebhook) Default(ctx context.Context, obj runtime.Object) error {
	coord := obj.(*spiderpoolv2beta1.SpiderCoordinator)

	logger := WebhookLogger.Named("Mutating").With(
		zap.String("CoordinatorName", coord.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Sugar().Debugf("Request Coordinator: %+v", *coord)

	if err := mutateCoordinator(logutils.IntoContext(ctx, logger), coord); err != nil {
		logger.Sugar().Errorf("Failed to mutate Coordinator: %w", err)
	}

	return nil
}

var _ webhook.CustomValidator = (*CoordinatorWebhook)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (cw *CoordinatorWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	coord := obj.(*spiderpoolv2beta1.SpiderCoordinator)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("CoordinatorName", coord.Name),
		zap.String("Operation", "CREATE"),
	)
	logger.Sugar().Debugf("Request Coordinator: %+v", *coord)

	if errs := validateCreateCoordinator(coord); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to create Coordinator: %w", errs.ToAggregate().Error())
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.KindSpiderCoordinator},
			coord.Name,
			errs,
		)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (cw *CoordinatorWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldCoord := oldObj.(*spiderpoolv2beta1.SpiderCoordinator)
	newCoord := newObj.(*spiderpoolv2beta1.SpiderCoordinator)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("CoordinatorName", newCoord.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old Coordinator: %+v", *oldCoord)
	logger.Sugar().Debugf("Request new Coordinator: %+v", *newCoord)

	if errs := validateUpdateCoordinator(oldCoord, newCoord); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to update Coordinator: %w", errs.ToAggregate().Error())
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.KindSpiderCoordinator},
			newCoord.Name,
			errs,
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (cw *CoordinatorWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
