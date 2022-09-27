// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var webhookLogger *zap.Logger

func (im *ipPoolManager) SetupWebhook() error {
	webhookLogger = logutils.Logger.Named("IPPool-Webhook")

	return ctrl.NewWebhookManagedBy(im.runtimeMgr).
		For(&spiderpoolv1.SpiderIPPool{}).
		WithDefaulter(im).
		WithValidator(im).
		Complete()
}

var _ webhook.CustomDefaulter = (*ipPoolManager)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (im *ipPoolManager) Default(ctx context.Context, obj runtime.Object) error {
	ipPool, ok := obj.(*spiderpoolv1.SpiderIPPool)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("mutating webhook of IPPool got an object with mismatched GVK: %+v", obj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Mutating").With(
		zap.String("IPPoolName", ipPool.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Info("Start to mutate IPPool")
	logger.Sugar().Debugf("Request IPPool: %+v", *ipPool)

	if ipPool.DeletionTimestamp != nil {
		logger.Info("Deleting IPPool, noting to mutate")
		return nil
	}

	if ipPool.Spec.IPVersion == nil {
		var version types.IPVersion
		if spiderpoolip.IsIPv4CIDR(ipPool.Spec.Subnet) {
			version = constant.IPv4
		} else if spiderpoolip.IsIPv6CIDR(ipPool.Spec.Subnet) {
			version = constant.IPv6
		} else {
			logger.Error("Invalid 'spec.ipVersion', noting to mutate")
			return nil
		}

		ipPool.Spec.IPVersion = new(types.IPVersion)
		*ipPool.Spec.IPVersion = version
		logger.Sugar().Infof("Set 'spec.ipVersion' to %d", version)
	}

	if len(ipPool.Spec.IPs) > 1 {
		mergedIPs, err := spiderpoolip.MergeIPRanges(*ipPool.Spec.IPVersion, ipPool.Spec.IPs)
		if err != nil {
			logger.Sugar().Errorf("Failed to merge 'spec.ips': %v", err)
		} else {
			ipPool.Spec.IPs = mergedIPs
			logger.Sugar().Debugf("Merge 'spec.ips':\n%v\n\nto:\n\n%v", ipPool.Spec.IPs, mergedIPs)
		}
	}

	if len(ipPool.Spec.ExcludeIPs) > 1 {
		mergedExcludeIPs, err := spiderpoolip.MergeIPRanges(*ipPool.Spec.IPVersion, ipPool.Spec.ExcludeIPs)
		if err != nil {
			logger.Sugar().Errorf("Failed to merge 'spec.excludeIPs': %v", err)
		} else {
			ipPool.Spec.ExcludeIPs = mergedExcludeIPs
			logger.Sugar().Debugf("Merge 'spec.excludeIPs':\n%v\n\nto:\n\n%v", ipPool.Spec.ExcludeIPs, mergedExcludeIPs)
		}
	}

	if !controllerutil.ContainsFinalizer(ipPool, constant.SpiderFinalizer) {
		controllerutil.AddFinalizer(ipPool, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer %s", constant.SpiderFinalizer)
	}

	return nil
}

var _ webhook.CustomValidator = (*ipPoolManager)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	ipPool, ok := obj.(*spiderpoolv1.SpiderIPPool)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("validating webhook of IPPool got an object with mismatched GVK: %+v", obj.GetObjectKind().GroupVersionKind()))
	}
	if im.config.EnableSpiderSubnet && im.subnetManager == nil {
		return apierrors.NewInternalError(errors.New("subnet manager must be injected when the feature SpiderSubnet is enabled"))
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("IPPoolName", ipPool.Name),
		zap.String("Operation", "CREATE"),
	)
	logger.Sugar().Debugf("Request IPPool: %+v", *ipPool)

	if errs := im.validateCreateIPPoolAndUpdateSubnetFreeIPs(ctx, ipPool); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to create IPPool: %v", errs.ToAggregate().Error())
		return apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.SpiderIPPoolKind},
			ipPool.Name,
			errs,
		)
	}

	return nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldIPPool, _ := oldObj.(*spiderpoolv1.SpiderIPPool)
	newIPPool, ok := newObj.(*spiderpoolv1.SpiderIPPool)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("validating webhook of IPPool got an object with mismatched GVK: %+v", newObj.GetObjectKind().GroupVersionKind()))
	}
	if im.config.EnableSpiderSubnet && im.subnetManager == nil {
		return apierrors.NewInternalError(errors.New("subnet manager must be injected when the feature SpiderSubnet is enabled"))
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("IPPoolName", newIPPool.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old IPPool: %+v", *oldIPPool)
	logger.Sugar().Debugf("Request new IPPool: %+v", *newIPPool)

	if errs := im.validateUpdateIPPoolAndUpdateSubnetFreeIPs(ctx, oldIPPool, newIPPool); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to update IPPool: %v", errs.ToAggregate().Error())
		return apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.SpiderIPPoolKind},
			newIPPool.Name,
			errs,
		)
	}

	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
