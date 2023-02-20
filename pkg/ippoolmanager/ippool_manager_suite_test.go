// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/golang/mock/gomock"
	electionmock "github.com/spidernet-io/spiderpool/pkg/election/mock"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	ripmanagermock "github.com/spidernet-io/spiderpool/pkg/reservedipmanager/mock"
	corev1 "k8s.io/api/core/v1"
)

var scheme *runtime.Scheme
var fakeClient client.Client
var ipPoolWebhook *ippoolmanager.IPPoolWebhook
var ipPoolManager ippoolmanager.IPPoolManager
var rIPManagerMock *ripmanagermock.MockReservedIPManager
var rIPManager reservedipmanager.ReservedIPManager

var mockCtrl *gomock.Controller
var mockLeaderElector *electionmock.MockSpiderLeaseElector

func TestIPPoolManager(t *testing.T) {
	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()

	RegisterFailHandler(Fail)
	RunSpecs(t, "IPPoolManager Suite", Label("ippoolmanager", "unitest"))
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

	ipPoolWebhook = &ippoolmanager.IPPoolWebhook{
		Client:             fakeClient,
		Scheme:             scheme,
		EnableIPv4:         true,
		EnableIPv6:         true,
		EnableSpiderSubnet: true,
	}

	rIPManager, err = reservedipmanager.NewReservedIPManager(fakeClient)
	Expect(err).NotTo(HaveOccurred())

	ipPoolManager, err = ippoolmanager.NewIPPoolManager(ippoolmanager.IPPoolManagerConfig{
		MaxConflictRetries:    3,
		ConflictRetryUnitTime: time.Second,
	}, fakeClient, rIPManager)
	Expect(err).NotTo(HaveOccurred())

	mockLeaderElector = electionmock.NewMockSpiderLeaseElector(mockCtrl)
})
