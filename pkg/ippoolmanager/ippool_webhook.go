// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"time"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/strings/slices"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
)

var webhookLogger = logutils.Logger.Named("IPPool-Webhook")

func (im *ipPoolManager) SetupWebhook() error {
	return ctrl.NewWebhookManagedBy(im.runtimeMgr).
		For(&spiderpoolv1.IPPool{}).
		WithDefaulter(im).
		WithValidator(im).
		Complete()
}

var _ webhook.CustomDefaulter = (*ipPoolManager)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (im *ipPoolManager) Default(ctx context.Context, obj runtime.Object) error {
	ipPool, ok := obj.(*spiderpoolv1.IPPool)
	if !ok {
		return fmt.Errorf("mutating webhook of IPPool got an object with mismatched type: %+v", obj.GetObjectKind().GroupVersionKind())
	}
	logger := webhookLogger.Named("Mutating").With(
		zap.String("IPPoolNamespace", ipPool.Namespace),
		zap.String("IPPoolName", ipPool.Name),
		zap.String("Operation", "DEFAULT"),
	)
	logger.Sugar().Debugf("Request IPPool: %+v", *ipPool)

	// do not mutate when it's a deleting object
	if ipPool.DeletionTimestamp != nil {
		return nil
	}

	if ipPool.Spec.IPVersion == nil {
		ipPool.Spec.IPVersion = new(types.IPVersion)

		var version types.IPVersion
		if spiderpoolip.IsIPv4CIDR(ipPool.Spec.Subnet) {
			version = constant.IPv4
		} else if spiderpoolip.IsIPv6CIDR(ipPool.Spec.Subnet) {
			version = constant.IPv6
		}

		*ipPool.Spec.IPVersion = version
		logger.Sugar().Infof("Set ipVersion '%s'", version)
	}

	// add finalizer for IPPool CR
	if !slices.Contains(ipPool.Finalizers, constant.SpiderFinalizer) {
		ipPool.Finalizers = append(ipPool.Finalizers, constant.SpiderFinalizer)
		logger.Sugar().Infof("Add finalizer '%s'", constant.SpiderFinalizer)
	}

	return nil
}

var _ webhook.CustomValidator = (*ipPoolManager)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	ipPool, ok := obj.(*spiderpoolv1.IPPool)
	if !ok {
		return fmt.Errorf("validating webhook of IPPool got an object with mismatched type: %+v", obj.GetObjectKind().GroupVersionKind())
	}
	logger := webhookLogger.Named("Validating").With(
		zap.String("Operation", "CREATE"),
		zap.String("IPPoolNamespace", ipPool.Namespace),
		zap.String("IPPoolName", ipPool.Name),
	)
	logger.Sugar().Debugf("Request IPPool: %+v", *ipPool)

	if err := im.validateCreateIPPool(ctx, ipPool); err != nil {
		logger.Sugar().Errorf("Failed to validate: %v", err)
		return err
	}

	return nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldIPPool, _ := oldObj.(*spiderpoolv1.IPPool)
	newIPPool, ok := newObj.(*spiderpoolv1.IPPool)
	if !ok {
		return fmt.Errorf("validating webhook of IPPool got an object with mismatched type: %+v", newObj.GetObjectKind().GroupVersionKind())
	}
	logger := webhookLogger.Named("Validating").With(
		zap.String("IPPoolNamespace", newIPPool.Namespace),
		zap.String("IPPoolName", newIPPool.Name),
		zap.String("Operation", "UPDATE"),
	)
	logger.Sugar().Debugf("Request old IPPool: %+v", *oldIPPool)
	logger.Sugar().Debugf("Request new IPPool: %+v", *newIPPool)

	if err := im.validateUpdateIPPool(ctx, oldIPPool, newIPPool); err != nil {
		logger.Sugar().Errorf("Failed to validate: %v", err)
		return err
	}

	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (im *ipPoolManager) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

// WebhookHealthyCheck servers for spiderpool controller readiness and liveness probe
func WebhookHealthyCheck(webhookPort string) error {
	// TODO (Icarus9913): Do we still need custom timeout seconds?
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	conn, err := tls.DialWithDialer(dialer, "tcp", net.JoinHostPort("", webhookPort), &tls.Config{InsecureSkipVerify: true})
	if err != nil {
		return fmt.Errorf("webhook server is not reachable: %w", err)
	}

	// close connection
	if err := conn.Close(); err != nil {
		return fmt.Errorf("webhook server is not reachable: closing connection: %w", err)
	}

	return nil
}
