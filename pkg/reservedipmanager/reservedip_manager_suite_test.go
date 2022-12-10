// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	mock "github.com/spidernet-io/spiderpool/pkg/reservedipmanager/mock"
)

var mockCtrl *gomock.Controller
var mockRIPManager *mock.MockReservedIPManager

var scheme *runtime.Scheme
var fakeClient client.Client
var rIPManager reservedipmanager.ReservedIPManager
var rIPWebhook *reservedipmanager.ReservedIPWebhook

func TestReservedIPManager(t *testing.T) {
	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()

	RegisterFailHandler(Fail)
	RunSpecs(t, "ReservedIPManager Suite", Label("reservedipmanager", "unitest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := spiderpoolv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	rIPManager, err = reservedipmanager.NewReservedIPManager(fakeClient)
	Expect(err).NotTo(HaveOccurred())

	mockRIPManager = mock.NewMockReservedIPManager(mockCtrl)
	rIPWebhook = &reservedipmanager.ReservedIPWebhook{
		ReservedIPManager: mockRIPManager,
	}
})
