// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager_test

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kruiseapi "github.com/openkruise/kruise-api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	electionmock "github.com/spidernet-io/spiderpool/pkg/election/mock"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	reservedipmanagermock "github.com/spidernet-io/spiderpool/pkg/reservedipmanager/mock"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
)

var (
	mockCtrl          *gomock.Controller
	mockLeaderElector *electionmock.MockSpiderLeaseElector
	mockRIPManager    *reservedipmanagermock.MockReservedIPManager
)

var (
	scheme            *runtime.Scheme
	fakeClient        client.Client
	tracker           k8stesting.ObjectTracker
	fakeAPIReader     client.Reader
	fakeDynamicClient dynamic.Interface
	subnetManager     subnetmanager.SubnetManager
	subnetWebhook     *subnetmanager.SubnetWebhook
)

func TestSubnetManager(t *testing.T) {
	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()

	RegisterFailHandler(Fail)
	RunSpecs(t, "SubnetManager Suite", Label("subnetmanager", "unittest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := spiderpoolv2beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())
	err = kruiseapi.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&spiderpoolv2beta1.SpiderSubnet{}, metav1.ObjectNameField, func(raw client.Object) []string {
			subnet := raw.(*spiderpoolv2beta1.SpiderSubnet)
			return []string{subnet.GetObjectMeta().GetName()}
		}).
		WithStatusSubresource(&spiderpoolv2beta1.SpiderSubnet{}).
		Build()

	_, err = metric.InitMetric(context.TODO(), constant.SpiderpoolController, false, false)
	Expect(err).NotTo(HaveOccurred())
	err = metric.InitSpiderpoolControllerMetrics(context.TODO())
	Expect(err).NotTo(HaveOccurred())

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&spiderpoolv2beta1.SpiderSubnet{}, metav1.ObjectNameField, func(raw client.Object) []string {
			subnet := raw.(*spiderpoolv2beta1.SpiderSubnet)
			return []string{subnet.GetObjectMeta().GetName()}
		}).
		WithStatusSubresource(&spiderpoolv2beta1.SpiderSubnet{}).
		Build()

	fakeDynamicClient = dynamicfake.NewSimpleDynamicClient(scheme)

	mockLeaderElector = electionmock.NewMockSpiderLeaseElector(mockCtrl)
	mockRIPManager = reservedipmanagermock.NewMockReservedIPManager(mockCtrl)
	subnetManager, err = subnetmanager.NewSubnetManager(
		fakeClient,
		fakeAPIReader,
		mockRIPManager,
	)
	Expect(err).NotTo(HaveOccurred())

	subnetWebhook = &subnetmanager.SubnetWebhook{
		Client:    fakeClient,
		APIReader: fakeAPIReader,
	}
})
