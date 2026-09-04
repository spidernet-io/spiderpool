// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	apitypes "k8s.io/apimachinery/pkg/types"
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

	EnableIPv4                              bool
	EnableIPv6                              bool
	EnableSpiderSubnet                      bool
	EnableValidatingResourcesDeletedWebhook bool
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
		logger.Sugar().Errorf("Failed to mutate IPPool: %w", err)
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
		logger.Sugar().Errorf("Failed to update IPPool: %w", errs.ToAggregate().Error())
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
	if !iw.EnableValidatingResourcesDeletedWebhook {
		return nil, nil
	}

	ipPool := obj.(*spiderpoolv2beta1.SpiderIPPool)

	logger := WebhookLogger.Named("Validating").With(
		zap.String("IPPoolName", ipPool.Name),
		zap.String("Operation", "DELETE"),
	)
	logger.Sugar().Debugf("Request IPPool: %+v", *ipPool)

	if ipPool.Status.AllocatedIPCount != nil && *ipPool.Status.AllocatedIPCount > 0 {
		logger.Sugar().Errorf("Cannot delete an IPPool with allocated IPs")
		return nil, apierrors.NewForbidden(
			schema.GroupResource{Group: constant.SpiderpoolAPIGroup, Resource: "spiderippools"},
			ipPool.Name,
			fmt.Errorf("cannot delete an IPPool with allocated IPs(%v)", *ipPool.Status.AllocatedIPCount),
		)
	}

	if err := iw.validateDeletePairedSiblingPool(ctx, logger, ipPool); err != nil {
		return nil, err
	}
	return nil, nil
}

// validateDeletePairedSiblingPool guards the deletion of the sibling v6 pool
// of a paired IaaS pool set. The v4 primary pool's provider-written
// status.ipMetaData records the paired IPv6 address of every prewarmed or
// bound entry, so deleting the v6 pool while such references still exist
// would strand them. The deletion is allowed when the v4 primary pool no
// longer exists, or when its metadata carries no IPv6 references; it is
// rejected while any metadata entry still references an IPv6 address.
func (iw *IPPoolWebhook) validateDeletePairedSiblingPool(ctx context.Context, logger *zap.Logger, ipPool *spiderpoolv2beta1.SpiderIPPool) error {
	if ipPool.Spec.IPVersion == nil || *ipPool.Spec.IPVersion != constant.IPv6 || !IsIaaSPool(ipPool) {
		return nil
	}
	pairName := ipPool.Annotations[constant.AnnoIPPoolPairPool]
	if pairName == "" {
		return nil
	}

	var v4Pool spiderpoolv2beta1.SpiderIPPool
	if err := iw.APIReader.Get(ctx, apitypes.NamespacedName{Name: pairName}, &v4Pool); err != nil {
		if apierrors.IsNotFound(err) {
			// The v4 primary pool is gone: nothing can reference this v6
			// pool's addresses anymore, so it is safe to delete.
			logger.Sugar().Infof("The pair IPPool %s of the sibling v6 IPPool %s no longer exists, allow the deletion", pairName, ipPool.Name)
			return nil
		}
		return apierrors.NewInternalError(fmt.Errorf("failed to get the pair IPPool %s of the sibling v6 IPPool %s: %w", pairName, ipPool.Name, err))
	}

	entries, err := MetadataIPEntriesFromPool(&v4Pool)
	if err != nil {
		// Fail closed: unreadable metadata cannot prove that no IPv6
		// address is still prewarmed or bound.
		logger.Sugar().Errorf("Cannot delete the sibling v6 IPPool %s: %v", ipPool.Name, err)
		return apierrors.NewForbidden(
			schema.GroupResource{Group: constant.SpiderpoolAPIGroup, Resource: "spiderippools"},
			ipPool.Name,
			fmt.Errorf("cannot delete the sibling v6 IPPool of pair IPPool %s whose status.ipMetaData is unreadable: %w", v4Pool.Name, err),
		)
	}
	if referenced := MetadataReferencedIPv6Set(entries); len(referenced) > 0 {
		logger.Sugar().Errorf("Cannot delete the sibling v6 IPPool %s: pair IPPool %s metadata still references %d IPv6 address(es)", ipPool.Name, v4Pool.Name, len(referenced))
		return apierrors.NewForbidden(
			schema.GroupResource{Group: constant.SpiderpoolAPIGroup, Resource: "spiderippools"},
			ipPool.Name,
			fmt.Errorf("cannot delete the sibling v6 IPPool: the pair IPPool %s status.ipMetaData still references %d prewarmed or bound IPv6 address(es)", v4Pool.Name, len(referenced)),
		)
	}
	return nil
}
