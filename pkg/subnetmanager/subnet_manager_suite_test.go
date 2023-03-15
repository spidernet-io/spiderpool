// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager_test

import (
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	electionmock "github.com/spidernet-io/spiderpool/pkg/election/mock"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
)

var mockCtrl *gomock.Controller
var mockLeaderElector *electionmock.MockSpiderLeaseElector

var scheme *runtime.Scheme
var fakeClient client.Client
var tracker k8stesting.ObjectTracker
var fakeAPIReader client.Reader
var subnetWebhook *subnetmanager.SubnetWebhook

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

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&spiderpoolv1.SpiderSubnet{}, metav1.ObjectNameField, func(raw client.Object) []string {
			subnet := raw.(*spiderpoolv1.SpiderSubnet)
			return []string{subnet.GetObjectMeta().GetName()}
		}).
		WithIndex(&spiderpoolv1.SpiderSubnet{}, "spec.default", func(raw client.Object) []string {
			subnet := raw.(*spiderpoolv1.SpiderSubnet)
			return []string{strconv.FormatBool(*subnet.Spec.Default)}
		}).
		Build()

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&spiderpoolv1.SpiderSubnet{}, metav1.ObjectNameField, func(raw client.Object) []string {
			subnet := raw.(*spiderpoolv1.SpiderSubnet)
			return []string{subnet.GetObjectMeta().GetName()}
		}).
		WithIndex(&spiderpoolv1.SpiderSubnet{}, "spec.default", func(raw client.Object) []string {
			subnet := raw.(*spiderpoolv1.SpiderSubnet)
			return []string{strconv.FormatBool(*subnet.Spec.Default)}
		}).
		Build()

	subnetWebhook = &subnetmanager.SubnetWebhook{
		Client:    fakeClient,
		APIReader: fakeAPIReader,
	}

	mockLeaderElector = electionmock.NewMockSpiderLeaseElector(mockCtrl)
})
