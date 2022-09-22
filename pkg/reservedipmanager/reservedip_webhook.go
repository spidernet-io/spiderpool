// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var webhookLogger *zap.Logger

func (rm *reservedIPManager) SetupWebhook() error {
	if webhookLogger == nil {
		webhookLogger = logutils.Logger.Named("ReservedIP-Webhook")
	}

	return ctrl.NewWebhookManagedBy(rm.runtimeMgr).
		For(&spiderpoolv1.SpiderReservedIP{}).
		WithDefaulter(rm).
		WithValidator(rm).
		Complete()
}

var _ webhook.CustomDefaulter = (*reservedIPManager)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (rm *reservedIPManager) Default(ctx context.Context, obj runtime.Object) error {
	rIP, ok := obj.(*spiderpoolv1.SpiderReservedIP)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("mutating webhook of ReservedIP got an object with mismatched GVK: %+v", obj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Mutating").With(
		zap.String("ReservedIPName", rIP.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Info("Start to mutate ReservedIP")
	logger.Sugar().Debugf("Request ReservedIP: %+v", *rIP)

	if rIP.DeletionTimestamp != nil {
		logger.Info("Deleting ReservedIP, noting to mutate")
		return nil
	}

	if len(rIP.Spec.IPs) == 0 {
		logger.Error("Empty 'spec.ips', noting to mutate")
		return nil
	}

	if rIP.Spec.IPVersion == nil {
		var version types.IPVersion
		if spiderpoolip.IsIPv4IPRange(rIP.Spec.IPs[0]) {
			version = constant.IPv4
		} else if spiderpoolip.IsIPv6IPRange(rIP.Spec.IPs[0]) {
			version = constant.IPv6
		} else {
			logger.Error("Invalid 'spec.ipVersion', noting to mutate")
			return nil
		}

		rIP.Spec.IPVersion = new(types.IPVersion)
		*rIP.Spec.IPVersion = version
		logger.Sugar().Infof("Set 'spec.ipVersion' to %d", version)
	}

	if len(rIP.Spec.IPs) > 1 {
		mergedIPs, err := spiderpoolip.MergeIPRanges(*rIP.Spec.IPVersion, rIP.Spec.IPs)
		if err != nil {
			logger.Sugar().Errorf("Failed to merge 'spec.ips': %v", err)
		} else {
			rIP.Spec.IPs = mergedIPs
			logger.Sugar().Debugf("Merge 'spec.ips':\n%v\n\nto:\n\n%v", rIP.Spec.IPs, mergedIPs)
		}
	}

	return nil
}

var _ webhook.CustomValidator = (*reservedIPManager)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (rm *reservedIPManager) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	rIP, ok := obj.(*spiderpoolv1.SpiderReservedIP)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("validating webhook of ReservedIP got an object with mismatched GVK: %+v", obj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("ReservedIPNamespace", rIP.Namespace),
		zap.String("ReservedIPName", rIP.Name),
		zap.String("Operation", "CREATE"),
	)
	logger.Sugar().Debugf("Request ReservedIP: %+v", *rIP)

	if errs := rm.validateCreateReservedIP(ctx, rIP); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to create ReservedIP: %v", errs.ToAggregate().Error())
		return apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.SpiderReservedIPKind},
			rIP.Name,
			errs,
		)
	}

	return nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (rm *reservedIPManager) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldRIP, _ := oldObj.(*spiderpoolv1.SpiderReservedIP)
	newRIP, ok := newObj.(*spiderpoolv1.SpiderReservedIP)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("validating webhook of ReservedIP got an object with mismatched GVK: %+v", newObj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("ReservedIPNamespace", newRIP.Namespace),
		zap.String("ReservedIPName", newRIP.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old ReservedIP: %+v", *oldRIP)
	logger.Sugar().Debugf("Request new ReservedIP: %+v", *newRIP)

	if errs := rm.validateUpdateReservedIP(ctx, oldRIP, newRIP); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to update ReservedIP: %v", errs.ToAggregate().Error())
		return apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.SpiderReservedIPKind},
			newRIP.Name,
			errs,
		)
	}

	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (rm *reservedIPManager) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
