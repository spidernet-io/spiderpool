// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

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

type SubnetWebhook struct {
	Client    client.Client
	APIReader client.Reader

	EnableIPv4 bool
	EnableIPv6 bool
}

func (sw *SubnetWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if WebhookLogger == nil {
		WebhookLogger = logutils.Logger.Named("Subnet-Webhook")
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv2beta1.SpiderSubnet{}).
		WithDefaulter(sw).
		WithValidator(sw).
		Complete()
}

var _ webhook.CustomDefaulter = (*SubnetWebhook)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (sw *SubnetWebhook) Default(ctx context.Context, obj runtime.Object) error {
	subnet := obj.(*spiderpoolv2beta1.SpiderSubnet)

	logger := WebhookLogger.Named("Mutating").With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Sugar().Debugf("Request Subnet: %+v", *subnet)

	if err := sw.mutateSubnet(logutils.IntoContext(ctx, logger), subnet); err != nil {
		logger.Sugar().Errorf("Failed to mutate Subnet: %v", err)
	}

	return nil
}

var _ webhook.CustomValidator = (*SubnetWebhook)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (sw *SubnetWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	subnet := obj.(*spiderpoolv2beta1.SpiderSubnet)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "CREATE"),
	)
	logger.Sugar().Debugf("Request Subnet: %+v", *subnet)

	if errs := sw.validateCreateSubnet(logutils.IntoContext(ctx, logger), subnet); len(errs) != 0 {
		aggregatedErr := errs.ToAggregate()
		logger.Sugar().Errorf("Failed to create Subnet: %s", aggregatedErr)
		// the user will receive the following errors rather than K8S API server specific typed errors.
		// Refer to https://github.com/spidernet-io/spiderpool/issues/3321
		switch {
		case strings.Contains(aggregatedErr.Error(), string(metav1.StatusReasonAlreadyExists)):
			return nil, apierrors.NewAlreadyExists(spiderpoolv2beta1.Resource(constant.KindSpiderSubnet), subnet.Name)
		default:
			return nil, apierrors.NewInvalid(
				schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.KindSpiderSubnet},
				subnet.Name,
				errs,
			)
		}
	}

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (sw *SubnetWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldSubnet := oldObj.(*spiderpoolv2beta1.SpiderSubnet)
	newSubnet := newObj.(*spiderpoolv2beta1.SpiderSubnet)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("SubnetName", newSubnet.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old Subnet: %+v", *oldSubnet)
	logger.Sugar().Debugf("Request new Subnet: %+v", *newSubnet)

	if newSubnet.DeletionTimestamp != nil {
		if !controllerutil.ContainsFinalizer(newSubnet, constant.SpiderFinalizer) {
			return nil, nil
		}

		return nil, apierrors.NewForbidden(
			schema.GroupResource{},
			"",
			errors.New("cannot update a terminaing Subnet"),
		)
	}

	if errs := sw.validateUpdateSubnet(logutils.IntoContext(ctx, logger), oldSubnet, newSubnet); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to update Subnet: %v", errs.ToAggregate().Error())
		return nil, apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.KindSpiderSubnet},
			newSubnet.Name,
			errs,
		)
	}

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (sw *SubnetWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
