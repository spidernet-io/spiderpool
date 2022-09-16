// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager

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

func (sm *subnetManager) SetupWebhook() error {
	webhookLogger = logutils.Logger.Named("Subnet-Webhook")

	return ctrl.NewWebhookManagedBy(sm.runtimeMgr).
		For(&spiderpoolv1.SpiderSubnet{}).
		WithDefaulter(sm).
		WithValidator(sm).
		Complete()
}

var _ webhook.CustomDefaulter = (*subnetManager)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (sm *subnetManager) Default(ctx context.Context, obj runtime.Object) error {
	subnet, ok := obj.(*spiderpoolv1.SpiderSubnet)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("mutating webhook of Subnet got an object with mismatched GVK: %+v", obj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Mutating").With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Info("Start to mutate Subnet")
	logger.Sugar().Debugf("Request Subnet: %+v", *subnet)

	if subnet.DeletionTimestamp != nil {
		logger.Info("Deleting Subnet, noting to mutate")
		return nil
	}

	if subnet.Spec.IPVersion == nil {
		var version types.IPVersion
		if spiderpoolip.IsIPv4CIDR(subnet.Spec.Subnet) {
			version = constant.IPv4
		} else if spiderpoolip.IsIPv6CIDR(subnet.Spec.Subnet) {
			version = constant.IPv6
		} else {
			logger.Error("Invalid 'spec.ipVersion', noting to mutate")
			return nil
		}

		subnet.Spec.IPVersion = new(types.IPVersion)
		*subnet.Spec.IPVersion = version
		logger.Sugar().Infof("Set 'spec.ipVersion' to %d", version)
	}

	if len(subnet.Spec.IPs) > 1 {
		mergedIPs, err := spiderpoolip.MergeIPRanges(*subnet.Spec.IPVersion, subnet.Spec.IPs)
		if err != nil {
			logger.Sugar().Errorf("Failed to merge 'spec.ips': %v", err)
		} else {
			subnet.Spec.IPs = mergedIPs
			logger.Sugar().Debugf("Merge 'spec.ips':\n%v\n\nto:\n\n%v", subnet.Spec.IPs, mergedIPs)
		}
	}

	if len(subnet.Spec.ExcludeIPs) > 1 {
		mergedExcludeIPs, err := spiderpoolip.MergeIPRanges(*subnet.Spec.IPVersion, subnet.Spec.ExcludeIPs)
		if err != nil {
			logger.Sugar().Errorf("Failed to merge 'spec.excludeIPs': %v", err)
		} else {
			subnet.Spec.ExcludeIPs = mergedExcludeIPs
			logger.Sugar().Debugf("Merge 'spec.excludeIPs':\n%v\n\nto:\n\n%v", subnet.Spec.ExcludeIPs, mergedExcludeIPs)
		}
	}

	return nil
}

var _ webhook.CustomValidator = (*subnetManager)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (sm *subnetManager) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	subnet, ok := obj.(*spiderpoolv1.SpiderSubnet)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("validating webhook of Subnet got an object with mismatched GVK: %+v", obj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "CREATE"),
	)
	logger.Sugar().Debugf("Request Subnet: %+v", *subnet)

	if errs := sm.validateCreateSubnet(ctx, subnet); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to create Subnet: %v", errs.ToAggregate().Error())
		return apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.SpiderSubnetKind},
			subnet.Name,
			errs,
		)
	}

	return nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (sm *subnetManager) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldSubnet, _ := oldObj.(*spiderpoolv1.SpiderSubnet)
	newSubnet, ok := newObj.(*spiderpoolv1.SpiderSubnet)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("validating webhook of Subnet got an object with mismatched GVK: %+v", newObj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("SubnetName", newSubnet.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old Subnet: %+v", *oldSubnet)
	logger.Sugar().Debugf("Request new Subnet: %+v", *newSubnet)

	if errs := sm.validateUpdateSubnet(ctx, oldSubnet, newSubnet); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to update Subnet: %v", errs.ToAggregate().Error())
		return apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.SpiderSubnetKind},
			newSubnet.Name,
			errs,
		)
	}

	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (sm *subnetManager) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	subnet, ok := obj.(*spiderpoolv1.SpiderSubnet)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("validating webhook of Subnet got an object with mismatched GVK: %+v", obj.GetObjectKind().GroupVersionKind()))
	}

	logger := webhookLogger.Named("Validating").With(
		zap.String("SubnetName", subnet.Name),
		zap.String("Operation", "DELETE"),
	)
	logger.Sugar().Debugf("Request Subnet: %+v", *subnet)

	if errs := sm.validateDeleteSubnet(ctx, subnet); len(errs) != 0 {
		logger.Sugar().Errorf("Failed to delete Subnet: %v", errs.ToAggregate().Error())
		return apierrors.NewInvalid(
			schema.GroupKind{Group: constant.SpiderpoolAPIGroup, Kind: constant.SpiderSubnetKind},
			subnet.Name,
			errs,
		)
	}

	return nil
}
