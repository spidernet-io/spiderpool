// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package multuscniconfig

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var logger *zap.Logger

type MultusConfigWebhook struct {
	APIReader client.Reader
}

func (mcw *MultusConfigWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if logger == nil {
		logger = logutils.Logger.Named("MultusConfig-Webhook")
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv2beta1.SpiderMultusConfig{}).
		WithDefaulter(mcw).
		WithValidator(mcw).
		Complete()
}

var _ webhook.CustomDefaulter = (*MultusConfigWebhook)(nil)

// Default implements admission.CustomDefaulter.
func (*MultusConfigWebhook) Default(ctx context.Context, obj runtime.Object) error {
	smc := obj.(*spiderpoolv2beta1.SpiderMultusConfig)

	mutateLogger := logger.Named("Mutating").With(
		zap.String("SpiderMultusConfig", smc.Name))
	mutateLogger.Sugar().Debugf("Request SpiderMultusConfig: %+v", *smc)

	mutateSpiderMultusConfig(logutils.IntoContext(ctx, mutateLogger), smc)
	mutateLogger.Sugar().Debugf("Finish SpiderMultusConfig: %+v", smc)
	return nil
}

var _ webhook.CustomValidator = (*MultusConfigWebhook)(nil)

func (mcw *MultusConfigWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	multusConfig := obj.(*spiderpoolv2beta1.SpiderMultusConfig)

	log := logger.Named("Validating").With(
		zap.String("SpiderMultusConfig", fmt.Sprintf("%s/%s", multusConfig.Namespace, multusConfig.Name)),
		zap.String("Operation", "CREATE"),
	)
	log.Sugar().Debugf("Request SpiderMultusConfig: %+v", *multusConfig)

	err := mcw.validate(logutils.IntoContext(ctx, logger), nil, multusConfig)
	if nil != err {
		return nil, apierrors.NewInvalid(
			spiderpoolv2beta1.SchemeGroupVersion.WithKind(constant.KindSpiderMultusConfig).GroupKind(),
			multusConfig.Name,
			field.ErrorList{err},
		)
	}

	return nil, nil
}

func (mcw *MultusConfigWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	oldMultusConfig := oldObj.(*spiderpoolv2beta1.SpiderMultusConfig)
	newMultusConfig := newObj.(*spiderpoolv2beta1.SpiderMultusConfig)

	log := logger.Named("Validating").With(
		zap.String("SpiderMultusConfig", fmt.Sprintf("%s/%s", newMultusConfig.Namespace, newMultusConfig.Name)),
		zap.String("Operation", "UPDATE"),
	)
	log.Sugar().Debugf("Request old SpiderMultusConfig: %+v", *oldMultusConfig)
	log.Sugar().Debugf("Request new SpiderMultusConfig: %+v", *newMultusConfig)

	err := mcw.validate(logutils.IntoContext(ctx, logger), oldMultusConfig, newMultusConfig)
	if nil != err {
		return nil, apierrors.NewInvalid(
			spiderpoolv2beta1.SchemeGroupVersion.WithKind(constant.KindSpiderMultusConfig).GroupKind(),
			newMultusConfig.Name,
			field.ErrorList{err},
		)
	}

	return nil, nil
}

// ValidateDelete will implement something just like kubernetes Foreground cascade deletion to delete the MultusConfig corresponding net-attach-def firstly
// Since the MultusConf doesn't have Finalizer, you could delete it as soon as possible and we can't filter it to delete the net-attach-def at first.
func (mcw *MultusConfigWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
