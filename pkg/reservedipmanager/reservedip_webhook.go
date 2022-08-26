// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager

import (
	"context"
	"fmt"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
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
		return fmt.Errorf("mutating webhook of ReservedIP got an object with mismatched type: %+v", obj.GetObjectKind().GroupVersionKind())
	}

	logger := webhookLogger.Named("Mutating").With(
		zap.String("ReservedIPNamespace", rIP.Namespace),
		zap.String("ReservedIPName", rIP.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Sugar().Debugf("Request ReservedIP: %+v", *rIP)

	if rIP.DeletionTimestamp != nil {
		return nil
	}

	if rIP.Spec.IPVersion != nil {
		return nil
	}

	if len(rIP.Spec.IPs) == 0 {
		return nil
	}

	var version types.IPVersion
	if spiderpoolip.IsIPv4IPRange(rIP.Spec.IPs[0]) {
		version = constant.IPv4
	} else if spiderpoolip.IsIPv6IPRange(rIP.Spec.IPs[0]) {
		version = constant.IPv6
	}

	if version == constant.IPv4 || version == constant.IPv6 {
		rIP.Spec.IPVersion = new(types.IPVersion)
		*rIP.Spec.IPVersion = version
		logger.Sugar().Infof("Set ipVersion '%s'", version)
	}

	return nil
}

var _ webhook.CustomValidator = (*reservedIPManager)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (rm *reservedIPManager) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	rIP, ok := obj.(*spiderpoolv1.SpiderReservedIP)
	if !ok {
		return fmt.Errorf("validating webhook of ReservedIP got an object with mismatched type: %+v", obj.GetObjectKind().GroupVersionKind())
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("ReservedIPNamespace", rIP.Namespace),
		zap.String("ReservedIPName", rIP.Name),
		zap.String("Operation", "CREATE"),
	)
	logger.Sugar().Debugf("Request ReservedIP: %+v", *rIP)

	if err := rm.validateCreateReservedIP(ctx, rIP); err != nil {
		logger.Sugar().Errorf("Failed to validate: %v", err)
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (rm *reservedIPManager) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldRIP, _ := oldObj.(*spiderpoolv1.SpiderReservedIP)
	newRIP, ok := newObj.(*spiderpoolv1.SpiderReservedIP)
	if !ok {
		return fmt.Errorf("validating webhook of ReservedIP got an object with mismatched type: %+v", newObj.GetObjectKind().GroupVersionKind())
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("ReservedIPNamespace", newRIP.Namespace),
		zap.String("ReservedIPName", newRIP.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old ReservedIP: %+v", *oldRIP)
	logger.Sugar().Debugf("Request new ReservedIP: %+v", *newRIP)

	if err := rm.validateUpdateReservedIP(ctx, oldRIP, newRIP); err != nil {
		logger.Sugar().Errorf("Failed to validate: %v", err)
		return err
	}

	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (rm *reservedIPManager) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
