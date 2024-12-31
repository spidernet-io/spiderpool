// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package spidercliamparameter

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

type SpiderClaimParameterManager interface {
	admission.CustomDefaulter
	admission.CustomValidator
}

type spiderClaimParameterWebhook struct {
	Client    client.Client
	APIReader client.Reader
}

func New(client client.Client, apiReader client.Reader, mgr ctrl.Manager) error {
	if logger == nil {
		logger = logutils.Logger.Named("SpiderClaimParameter-Webhook")
	}

	scpm := &spiderClaimParameterWebhook{
		Client:    client,
		APIReader: apiReader,
	}

	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv2beta1.SpiderClaimParameter{}).
		WithDefaulter(scpm).
		WithValidator(scpm).
		Complete()
}

var _ webhook.CustomValidator = &spiderClaimParameterWebhook{}

// Default implements admission.CustomDefaulter.
func (*spiderClaimParameterWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (scpm *spiderClaimParameterWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	scp := obj.(*spiderpoolv2beta1.SpiderClaimParameter)

	log := logger.Named("Validating").With(
		zap.String("SpiderClaimParameter", fmt.Sprintf("%s/%s", scp.Namespace, scp.Name)),
		zap.String("Operation", "CREATE"),
	)
	log.Sugar().Debugf("Request SpiderClaimParameter: %+v", *scp)

	err := scpm.validate(scp)
	if err != nil {
		log.Error(err.Error())
		return nil, apierrors.NewInvalid(
			spiderpoolv2beta1.SchemeGroupVersion.WithKind(constant.KindSpiderClaimParameter).GroupKind(),
			scp.Name,
			field.ErrorList{err},
		)
	}

	return nil, nil
}

func (scpm *spiderClaimParameterWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	new := newObj.(*spiderpoolv2beta1.SpiderClaimParameter)

	log := logger.Named("Validating").With(
		zap.String("SpiderClaimParameter", fmt.Sprintf("%s/%s", new.Namespace, new.Name)),
		zap.String("Operation", "UPDATE"),
	)
	log.Sugar().Debugf("Request new SpiderClaimParameter: %+v", *new)

	err := scpm.validate(new)
	if err != nil {
		log.Error(err.Error())
		return nil, apierrors.NewInvalid(
			spiderpoolv2beta1.SchemeGroupVersion.WithKind(constant.KindSpiderClaimParameter).GroupKind(),
			new.Name,
			field.ErrorList{err},
		)
	}

	return nil, nil
}

// ValidateDelete will implement something just like kubernetes Foreground cascade deletion to delete the MultusConfig corresponding net-attach-def firstly
// Since the MultusConf doesn't have Finalizer, you could delete it as soon as possible and we can't filter it to delete the net-attach-def at first.
func (scpm *spiderClaimParameterWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// check the existence of SpiderMultusConfig in the StaticNics
func (scpm *spiderClaimParameterWebhook) validate(scp *spiderpoolv2beta1.SpiderClaimParameter) *field.Error {
	if scp == nil {
		return field.Required(nil, "SpiderClaimParameter is nil")
	}

	validateFunc := func(subPath string, nic *spiderpoolv2beta1.MultusConfig) *field.Error {
		if nic.MultusName == "" {
			return field.Invalid(field.NewPath("Spec").Child(fmt.Sprintf("%s.MultusName", subPath)), nic, "value should not be empty")
		}

		var smc spiderpoolv2beta1.SpiderMultusConfig
		if err := scpm.APIReader.Get(context.TODO(), client.ObjectKey{Name: nic.MultusName, Namespace: nic.Namespace}, &smc); err != nil {
			return field.Invalid(field.NewPath("Spec").Child(subPath), nic, fmt.Sprintf("error get spidermultusconfig %v: %v", nic, err))
		}
		return nil
	}

	if scp.Spec.DefaultNic != nil {
		if err := validateFunc("DefaultNic", scp.Spec.DefaultNic); err != nil {
			return err
		}
	}

	for idx, nic := range scp.Spec.SecondaryNics {
		if err := validateFunc(fmt.Sprintf("SecondaryNics[%d]", idx), &nic); err != nil {
			return err
		}
	}

	return nil
}
