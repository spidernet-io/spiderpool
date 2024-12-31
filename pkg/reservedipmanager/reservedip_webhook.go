// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"errors"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var WebhookLogger *zap.Logger

type ReservedIPWebhook struct {
	EnableIPv4 bool
	EnableIPv6 bool
}

func (rw *ReservedIPWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if WebhookLogger == nil {
		WebhookLogger = logutils.Logger.Named("ReservedIP-Webhook")
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv2beta1.SpiderReservedIP{}).
		WithDefaulter(rw).
		WithValidator(rw).
		Complete()
}

var _ webhook.CustomDefaulter = (*ReservedIPWebhook)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (rw *ReservedIPWebhook) Default(ctx context.Context, obj runtime.Object) error {
	rIP := obj.(*spiderpoolv2beta1.SpiderReservedIP)

	logger := WebhookLogger.Named("Mutating").With(
		zap.String("ReservedIPName", rIP.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Sugar().Debugf("Request ReservedIP: %+v", *rIP)

	if err := rw.mutateReservedIP(logutils.IntoContext(ctx, logger), rIP); err != nil {
		logger.Sugar().Errorf("Failed to mutate ReservedIP: %v", err)
	}

	return nil
}

var _ webhook.CustomValidator = (*ReservedIPWebhook)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (rw *ReservedIPWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	rIP := obj.(*spiderpoolv2beta1.SpiderReservedIP)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("ReservedIPNamespace", rIP.Namespace),
		zap.String("ReservedIPName", rIP.Name),
		zap.String("Operation", "CREATE"),
	)
	logger.Sugar().Debugf("Request ReservedIP: %+v", *rIP)

	if errs := rw.validateCreateReservedIP(logutils.IntoContext(ctx, logger), rIP); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to create ReservedIP: %v", errs.ToAggregate().Error())
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.KindSpiderReservedIP},
			rIP.Name,
			errs,
		)
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (rw *ReservedIPWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldRIP := oldObj.(*spiderpoolv2beta1.SpiderReservedIP)
	newRIP := newObj.(*spiderpoolv2beta1.SpiderReservedIP)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("ReservedIPNamespace", newRIP.Namespace),
		zap.String("ReservedIPName", newRIP.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old ReservedIP: %+v", *oldRIP)
	logger.Sugar().Debugf("Request new ReservedIP: %+v", *newRIP)

	if newRIP.DeletionTimestamp != nil {
		if oldRIP.DeletionTimestamp == nil {
			return nil, nil
		}

		return nil, apierrors.NewForbidden(
			schema.GroupResource{},
			"",
			errors.New("cannot update a terminating ReservedIP"),
		)
	}

	if errs := rw.validateUpdateReservedIP(logutils.IntoContext(ctx, logger), oldRIP, newRIP); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to update ReservedIP: %v", errs.ToAggregate().Error())
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.KindSpiderReservedIP},
			newRIP.Name,
			errs,
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (rw *ReservedIPWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
