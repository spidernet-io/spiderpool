// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package podmanager

import (
	"context"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	crdclientset "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var PodWebhookExcludeNamespaces = []string{
	metav1.NamespaceSystem,
	metav1.NamespacePublic,
	constant.Spiderpool,
	"metallb-system",
	"istio-system",
	// more system namespaces to be added
}

type PodWebhook interface {
	admission.CustomDefaulter
	admission.CustomValidator
}

type podWebhook struct {
	spiderClient crdclientset.Interface
}

// InitPodWebhook initializes the pod webhook.
// It sets up the mutating webhook for pods and registers it with the manager.
// Parameters:
//   - mgr: The controller manager
//
// Returns an error if initialization fails.
func InitPodWebhook(mgr ctrl.Manager) error {
	spiderClient, err := crdclientset.NewForConfig(ctrl.GetConfigOrDie())
	if err != nil {
		return err
	}

	pw := &podWebhook{
		spiderClient: spiderClient,
	}

	// setup mutating webhook for pods
	if err = ctrl.NewWebhookManagedBy(mgr).
		For(&corev1.Pod{}).
		WithDefaulter(pw).
		Complete(); err != nil {
		return err
	}
	return nil
}

// Default implements the defaulting webhook for pods.
// It injects network resources into the pod if it has the appropriate annotation.
// Parameters:
//   - ctx: The context
//   - obj: The runtime object (expected to be a Pod)
//
// Returns an error if defaulting fails.
func (pw *podWebhook) Default(ctx context.Context, obj runtime.Object) error {
	logger := logutils.FromContext(ctx)
	pod := obj.(*corev1.Pod)
	mutateLogger := logger.Named("PodMutating").With(
		zap.String("Pod", pod.GenerateName))
	mutateLogger.Sugar().Debugf("Request Pod: %+v", *pod)

	needInject := false
	for _, anno := range []string{constant.AnnoPodResourceInject, constant.AnnoNetworkResourceInject} {
		if _, ok := pod.Annotations[anno]; ok {
			mutateLogger.Sugar().Debugf("Pod %s/%s is annotated with %s, start injecting network resources", pod.Namespace, pod.GenerateName, anno)
			needInject = true
		}
	}

	if !needInject {
		return nil
	}

	err := podNetworkMutatingWebhook(pw.spiderClient, pod)
	if err != nil {
		mutateLogger.Sugar().Errorf("Failed to inject network resources for pod %s/%s: %v", pod.Namespace, pod.GenerateName, err)
		return err
	}
	mutateLogger.Sugar().Debugf("Pod %s/%s network resources injected, Pod: %v", pod.Namespace, pod.GenerateName, pod)
	return nil
}

// ValidateCreate implements the validation webhook for pod creation.
// Currently, it performs no validation and always returns nil.
func (pw *podWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateUpdate implements the validation webhook for pod updates.
// Currently, it performs no validation and always returns nil.
func (pw *podWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

// ValidateDelete implements the validation webhook for pod deletion.
// Currently, it performs no validation and always returns nil.
func (pw *podWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	return nil, nil
}
