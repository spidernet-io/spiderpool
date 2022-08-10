// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reservedipmanager_test

import (
	"context"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager/mocks"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var err error
var ctx context.Context
var cancel context.CancelFunc
var k8sClient client.Client
var scheme *runtime.Scheme
var rIPManager reservedipmanager.ReservedIPManager

func TestReservedipmanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Reservedipmanager Suite")
}

var _ = BeforeSuite(func() {
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	scheme = runtime.NewScheme()

	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = spiderpoolv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	mgr := &mocks.Manager{}

	// Create a fake client
	k8sClient = createFakeClient()
	// Define the method
	mgr.On("GetClient").Return(k8sClient).Maybe()
	mgr.On("GetScheme").Return(scheme).Maybe()

	rIPManager, err = reservedipmanager.NewReservedIPManager(k8sClient)
	Expect(err).NotTo(HaveOccurred())
	Expect(rIPManager).NotTo(BeNil())

})

func createFakeClient() client.Client {
	return fakeclient.NewClientBuilder().
		WithScheme(scheme).
		Build()
}
