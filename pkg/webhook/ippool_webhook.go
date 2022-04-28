// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// +kubebuilder:webhook:path=/mutate-spiderpool-spidernet-io-v1-ippool,mutating=true,failurePolicy=fail,sideEffects=None,groups=spiderpool.spidernet.io,resources=ippools,verbs=create;update,versions=v1,name=ippool-mutating.spiderpool.spidernet.io,admissionReviewVersions=v1
// +kubebuilder:webhook:path=/validate-spiderpool-spidernet-io-v1-ippool,mutating=false,failurePolicy=fail,sideEffects=None,groups=spiderpool.spidernet.io,resources=ippools,verbs=create;update;delete,versions=v1,name=ippool-validaing.spiderpool.spidernet.io,admissionReviewVersions=v1

package webhook

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
)

type IPPoolWebhook struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *IPPoolWebhook) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&spiderpoolv1.IPPool{}).
		WithDefaulter(r).
		WithValidator(r).
		Complete()
}

var _ webhook.CustomDefaulter = (*IPPoolWebhook)(nil)

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type
func (r *IPPoolWebhook) Default(ctx context.Context, obj runtime.Object) error {
	return nil
}

var _ webhook.CustomValidator = (*IPPoolWebhook)(nil)

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *IPPoolWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type
func (r *IPPoolWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	return nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type
func (r *IPPoolWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
