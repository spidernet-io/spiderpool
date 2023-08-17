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
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var logger *zap.Logger

type MultusConfigWebhook struct{}

func (mcw *MultusConfigWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	if logger == nil {
		logger = logutils.Logger.Named("MultusConfig-Webhook")
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv2beta1.SpiderMultusConfig{}).
		WithValidator(mcw).
		Complete()
}

var _ webhook.CustomValidator = &MultusConfigWebhook{}

func (mcw *MultusConfigWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	multusConfig := obj.(*spiderpoolv2beta1.SpiderMultusConfig)

	log := logger.Named("Validating").With(
		zap.String("MultusConfig", fmt.Sprintf("%s/%s", multusConfig.Namespace, multusConfig.Name)),
		zap.String("Operation", "CREATE"),
	)
	log.Sugar().Debugf("Request MultusConfig: %+v", *multusConfig)

	err := validate(nil, multusConfig)
	if nil != err {
		return apierrors.NewInvalid(
			spiderpoolv2beta1.SchemeGroupVersion.WithKind(constant.KindSpiderMultusConfig).GroupKind(),
			multusConfig.Name,
			field.ErrorList{err},
		)
	}

	return nil
}

func (mcw *MultusConfigWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldMultusConfig := oldObj.(*spiderpoolv2beta1.SpiderMultusConfig)
	newMultusConfig := newObj.(*spiderpoolv2beta1.SpiderMultusConfig)

	log := logger.Named("Validating").With(
		zap.String("MultusConfig", fmt.Sprintf("%s/%s", newMultusConfig.Namespace, newMultusConfig.Name)),
		zap.String("Operation", "UPDATE"),
	)
	log.Sugar().Debugf("Request old MultusConfig: %+v", *oldMultusConfig)
	log.Sugar().Debugf("Request new MultusConfig: %+v", *newMultusConfig)

	err := validate(oldMultusConfig, newMultusConfig)
	if nil != err {
		return apierrors.NewInvalid(
			spiderpoolv2beta1.SchemeGroupVersion.WithKind(constant.KindSpiderMultusConfig).GroupKind(),
			newMultusConfig.Name,
			field.ErrorList{err},
		)
	}

	return nil
}

// ValidateDelete will implement something just like kubernetes Foreground cascade deletion to delete the MultusConfig corresponding net-attach-def firstly
// Since the MultusConf doesn't have Finalizer, you could delete it as soon as possible and we can't filter it to delete the net-attach-def at first.
func (mcw *MultusConfigWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
