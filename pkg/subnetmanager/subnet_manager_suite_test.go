// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager_test

import (
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	electionmock "github.com/spidernet-io/spiderpool/pkg/election/mock"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	corev1 "k8s.io/api/core/v1"
)

var mockCtrl *gomock.Controller
var mockLeaderElector *electionmock.MockSpiderLeaseElector

var scheme *runtime.Scheme
var fakeClient client.Client
var subnetWebhook *subnetmanager.SubnetWebhook
var subnetManager subnetmanager.SubnetManager
var ipPoolManager ippoolmanager.IPPoolManager
var rIPManager reservedipmanager.ReservedIPManager

func TestSubnetManager(t *testing.T) {
	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()

	RegisterFailHandler(Fail)
	RunSpecs(t, "SubnetManager Suite", Label("subnetmanager", "unitest"))
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

	subnetWebhook = &subnetmanager.SubnetWebhook{
		Client: fakeClient,
	}

	mockLeaderElector = electionmock.NewMockSpiderLeaseElector(mockCtrl)

	rIPManager, err = reservedipmanager.NewReservedIPManager(fakeClient)
	Expect(err).NotTo(HaveOccurred())

	ipPoolManager, err = ippoolmanager.NewIPPoolManager(ippoolmanager.IPPoolManagerConfig{
		MaxConflictRetries: 3,
	}, fakeClient, rIPManager)
	Expect(err).NotTo(HaveOccurred())

	subnetManager, err = subnetmanager.NewSubnetManager(subnetmanager.SubnetManagerConfig{
		MaxConflictRetries: 3,
	}, fakeClient, ipPoolManager, fakeClient.Scheme())
	Expect(err).NotTo(HaveOccurred())
})
