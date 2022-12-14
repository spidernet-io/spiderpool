// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	podmock "github.com/spidernet-io/spiderpool/pkg/podmanager/mock"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var mockCtrl *gomock.Controller
var mockPodManager *podmock.MockPodManager

var scheme *runtime.Scheme
var fakeClient client.Client
var endpointManager workloadendpointmanager.WorkloadEndpointManager
var endpointWebhook *workloadendpointmanager.WorkloadEndpointWebhook

func TestWorkloadEndpointManager(t *testing.T) {
	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()

	RegisterFailHandler(Fail)
	RunSpecs(t, "WorkloadEndpointManager Suite", Label("workloadendpointmanager", "unitest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := spiderpoolv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	mockPodManager = podmock.NewMockPodManager(mockCtrl)
	endpointManager, err = workloadendpointmanager.NewWorkloadEndpointManager(
		workloadendpointmanager.EndpointManagerConfig{
			MaxConflictRetries: 1,
			MaxHistoryRecords:  pointer.Int(1),
		},
		fakeClient,
		mockPodManager,
	)
	Expect(err).NotTo(HaveOccurred())

	endpointWebhook = &workloadendpointmanager.WorkloadEndpointWebhook{}
})
