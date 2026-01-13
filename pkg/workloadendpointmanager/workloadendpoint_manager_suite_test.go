// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var (
	scheme          *runtime.Scheme
	fakeClient      client.Client
	tracker         k8stesting.ObjectTracker
	fakeAPIReader   client.Reader
	endpointManager workloadendpointmanager.WorkloadEndpointManager
)

func TestWorkloadEndpointManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WorkloadEndpointManager Suite", Label("workloadendpointmanager", "unittest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := spiderpoolv2beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&spiderpoolv2beta1.SpiderEndpoint{}, metav1.ObjectNameField, func(raw client.Object) []string {
			endpoint := raw.(*spiderpoolv2beta1.SpiderEndpoint)
			return []string{endpoint.GetObjectMeta().GetName()}
		}).
		Build()

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&spiderpoolv2beta1.SpiderEndpoint{}, metav1.ObjectNameField, func(raw client.Object) []string {
			endpoint := raw.(*spiderpoolv2beta1.SpiderEndpoint)
			return []string{endpoint.GetObjectMeta().GetName()}
		}).
		Build()

	endpointManager, err = workloadendpointmanager.NewWorkloadEndpointManager(
		fakeClient,
		fakeAPIReader,
		true,
		true,
	)
	Expect(err).NotTo(HaveOccurred())
})
