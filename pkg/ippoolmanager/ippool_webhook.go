// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"strings"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var WebhookLogger *zap.Logger

type IPPoolWebhook struct {
	Client    client.Client
	APIReader client.Reader

	EnableIPv4         bool
	EnableIPv6         bool
	EnableSpiderSubnet bool
}

func (iw *IPPoolWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if WebhookLogger == nil {
		WebhookLogger = logutils.Logger.Named("IPPool-Webhook")
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv2beta1.SpiderIPPool{}).
		WithDefaulter(iw).
		WithValidator(iw).
		Complete()
}

var _ webhook.CustomDefaulter = (*IPPoolWebhook)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (iw *IPPoolWebhook) Default(ctx context.Context, obj runtime.Object) error {
	ipPool := obj.(*spiderpoolv2beta1.SpiderIPPool)

	logger := WebhookLogger.Named("Mutating").With(
		zap.String("IPPoolName", ipPool.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Sugar().Debugf("Request IPPool: %+v", *ipPool)

	if err := iw.mutateIPPool(logutils.IntoContext(ctx, logger), ipPool); err != nil {
		logger.Sugar().Errorf("Failed to mutate IPPool: %v", err)
	}

	return nil
}

var _ webhook.CustomValidator = (*IPPoolWebhook)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (iw *IPPoolWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	ipPool := obj.(*spiderpoolv2beta1.SpiderIPPool)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("IPPoolName", ipPool.Name),
		zap.String("Operation", "CREATE"),
	)
	logger.Sugar().Debugf("Request IPPool: %+v", *ipPool)

	if errs := iw.validateCreateIPPoolWhileEnableSpiderSubnet(logutils.IntoContext(ctx, logger), ipPool); len(errs) != 0 {
		aggregatedErr := errs.ToAggregate()
		logger.Sugar().Errorf("Failed to create IPPool: %s", aggregatedErr)
		// the user will receive the following errors rather than K8S API server specific typed errors.
		// Refer to https://github.com/spidernet-io/spiderpool/issues/3321
		switch {
		case strings.Contains(aggregatedErr.Error(), string(metav1.StatusReasonAlreadyExists)):
			return nil, apierrors.NewAlreadyExists(spiderpoolv2beta1.Resource(constant.KindSpiderIPPool), ipPool.Name)
		default:
			return nil, apierrors.NewInvalid(
				schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.KindSpiderIPPool},
				ipPool.Name,
				errs,
			)
		}
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (iw *IPPoolWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldIPPool := oldObj.(*spiderpoolv2beta1.SpiderIPPool)
	newIPPool := newObj.(*spiderpoolv2beta1.SpiderIPPool)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("IPPoolName", newIPPool.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old IPPool: %v", oldIPPool)
	logger.Sugar().Debugf("Request new IPPool: %v", newIPPool)

	if newIPPool.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(newIPPool, constant.SpiderFinalizer) {
			return nil, nil
		}

		return nil, apierrors.NewForbidden(
			schema.GroupResource{},
			"",
			errors.New("cannot update a terminating IPPool"),
		)
	}

	if errs := iw.validateUpdateIPPoolWhileEnableSpiderSubnet(logutils.IntoContext(ctx, logger), oldIPPool, newIPPool); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to update IPPool: %v", errs.ToAggregate().Error())
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.KindSpiderIPPool},
			newIPPool.Name,
			errs,
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (iw *IPPoolWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
