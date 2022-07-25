// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"strings"

	"github.com/asaskevich/govalidator"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

var _ webhook.CustomDefaulter = (*ipPoolManager)(nil)

func (im *ipPoolManager) SetupWebhook(mgr ctrl.Manager) error {
	if mgr == nil {
		return fmt.Errorf("failed to set up IPPool webhook, mgr must be specified")
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv1.IPPool{}).
		WithDefaulter(im).
		WithValidator(im).
		Complete()
}

func (im *ipPoolManager) Default(ctx context.Context, obj runtime.Object) error {
	ipPool, ok := obj.(*spiderpoolv1.IPPool)
	if !ok {
		return fmt.Errorf("webhook mutating got mismatch IPPool type: %+v", obj.GetObjectKind().GroupVersionKind())
	}

	// do not mutate when it's a deleting object
	if ipPool.DeletionTimestamp != nil {
		return nil
	}

	if ipPool.Spec.IPVersion == nil {
		ipPool.Spec.IPVersion = new(spiderpoolv1.IPVersion)
		ip, _, _ := strings.Cut(ipPool.Spec.Subnet, "/")

		if govalidator.IsIPv4(ip) {
			*ipPool.Spec.IPVersion = spiderpoolv1.IPv4
			logger.Sugar().Infof("IPPool CR object '%s/%s' set ipVersion '%s'", ipPool.Namespace, ipPool.Name, spiderpoolv1.IPv4)
		} else if govalidator.IsIPv6(ip) {
			*ipPool.Spec.IPVersion = spiderpoolv1.IPv6
			logger.Sugar().Infof("IPPool CR object '%s/%s' set ipVersion '%s'", ipPool.Namespace, ipPool.Name, spiderpoolv1.IPv6)
		}
	}

	// add finalizer for IPPool CR
	if !slices.Contains(ipPool.Finalizers, constant.SpiderFinalizer) {
		ipPool.Finalizers = append(ipPool.Finalizers, constant.SpiderFinalizer)
		logger.Sugar().Infof("IPPool CR object '%s/%s' set finalizer '%s' successfully", ipPool.Namespace, ipPool.Name, constant.SpiderFinalizer)
	}

	return nil
}

var _ webhook.CustomValidator = (*ipPoolManager)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
