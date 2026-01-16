// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"context"
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

	"github.com/spidernet-io/spiderpool/pkg/constant"
	electionmock "github.com/spidernet-io/spiderpool/pkg/election/mock"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/metric"
	reservedipmanagermock "github.com/spidernet-io/spiderpool/pkg/reservedipmanager/mock"
)

var (
	mockCtrl          *gomock.Controller
	mockLeaderElector *electionmock.MockSpiderLeaseElector
	mockRIPManager    *reservedipmanagermock.MockReservedIPManager
)

var (
	scheme        *runtime.Scheme
	fakeClient    client.Client
	tracker       k8stesting.ObjectTracker
	fakeAPIReader client.Reader
	ipPoolManager ippoolmanager.IPPoolManager
	ipPoolWebhook *ippoolmanager.IPPoolWebhook
)

func TestIPPoolManager(t *testing.T) {
	mockCtrl = gomock.NewController(t)
	defer mockCtrl.Finish()

	RegisterFailHandler(Fail)
	RunSpecs(t, "IPPoolManager Suite", Label("ippoolmanager", "unittest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := spiderpoolv2beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&spiderpoolv2beta1.SpiderIPPool{}, metav1.ObjectNameField, func(raw client.Object) []string {
			ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
			return []string{ipPool.GetObjectMeta().GetName()}
		}).
		WithIndex(&spiderpoolv2beta1.SpiderIPPool{}, constant.SpecDefaultField, func(raw client.Object) []string {
			ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
			return []string{strconv.FormatBool(*ipPool.Spec.Default)}
		}).
		WithIndex(&spiderpoolv2beta1.SpiderIPPool{}, constant.SpecIPVersionField, func(raw client.Object) []string {
			ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
			if ipPool.Spec.IPVersion != nil {
				return []string{strconv.FormatInt(*ipPool.Spec.IPVersion, 10)}
			}
			return []string{}
		}).
		WithStatusSubresource(&spiderpoolv2beta1.SpiderIPPool{}).
		Build()
	_, err = metric.InitMetric(context.TODO(), constant.SpiderpoolAgent, false, false)
	Expect(err).NotTo(HaveOccurred())
	err = metric.InitSpiderpoolAgentMetrics(context.TODO(), nil)
	Expect(err).NotTo(HaveOccurred())

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&spiderpoolv2beta1.SpiderIPPool{}, metav1.ObjectNameField, func(raw client.Object) []string {
			ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
			return []string{ipPool.GetObjectMeta().GetName()}
		}).
		WithIndex(&spiderpoolv2beta1.SpiderIPPool{}, constant.SpecDefaultField, func(raw client.Object) []string {
			ipPool := raw.(*spiderpoolv2beta1.SpiderIPPool)
			return []string{strconv.FormatBool(*ipPool.Spec.Default)}
		}).
		WithStatusSubresource(&spiderpoolv2beta1.SpiderIPPool{}).
		Build()

	mockLeaderElector = electionmock.NewMockSpiderLeaseElector(mockCtrl)
	mockRIPManager = reservedipmanagermock.NewMockReservedIPManager(mockCtrl)
	ipPoolManager, err = ippoolmanager.NewIPPoolManager(
		ippoolmanager.IPPoolManagerConfig{
			EnableKubevirtStaticIP: true,
		},
		fakeClient,
		fakeAPIReader,
		mockRIPManager,
	)
	Expect(err).NotTo(HaveOccurred())

	ipPoolWebhook = &ippoolmanager.IPPoolWebhook{
		Client:             fakeClient,
		APIReader:          fakeAPIReader,
		EnableIPv4:         true,
		EnableIPv6:         true,
		EnableSpiderSubnet: true,
	}
})
