// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package utils_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	corev1 "k8s.io/api/core/v1"
	resourcev1alpha2 "k8s.io/api/resource/v1alpha2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var scheme *runtime.Scheme
var fakeClient client.Client
var tracker k8stesting.ObjectTracker
var fakeAPIReader client.Reader
var err error

func TestUtils(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite", Label("utils", "unittest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	_ = corev1.AddToScheme(scheme)
	_ = spiderpoolv2beta1.AddToScheme(scheme)
	_ = resourcev1alpha2.AddToScheme(scheme)

	fakeClient = fake.NewFakeClient(&spiderpoolv2beta1.SpiderClaimParameter{},
		&spiderpoolv2beta1.SpiderClaimParameterList{},
		&spiderpoolv2beta1.SpiderMultusConfigList{},
		&spiderpoolv2beta1.SpiderMultusConfig{},
	)

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&corev1.Pod{}, metav1.ObjectNameField, func(raw client.Object) []string {
			pod := raw.(*corev1.Pod)
			return []string{pod.GetObjectMeta().GetName()}
		}).
		WithIndex(&spiderpoolv2beta1.SpiderClaimParameter{}, metav1.ObjectNameField, func(raw client.Object) []string {
			scp := raw.(*spiderpoolv2beta1.SpiderClaimParameter)
			return []string{scp.GetObjectMeta().GetName()}
		}).
		WithIndex(&spiderpoolv2beta1.SpiderMultusConfig{}, metav1.ObjectNameField, func(raw client.Object) []string {
			smc := raw.(*spiderpoolv2beta1.SpiderMultusConfig)
			return []string{smc.GetObjectMeta().GetName()}
		}).
		Build()

	Expect(err).NotTo(HaveOccurred())
})
