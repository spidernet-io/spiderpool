// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
)

var scheme *runtime.Scheme
var fakeClient client.Client
var tracker k8stesting.ObjectTracker
var fakeAPIReader client.Reader
var ipPoolWebhook *ippoolmanager.IPPoolWebhook

func TestIPPoolManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "IPPoolManager Suite", Label("ippoolmanager", "unitest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := spiderpoolv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		Build()

	ipPoolWebhook = &ippoolmanager.IPPoolWebhook{
		Client:             fakeClient,
		APIReader:          fakeAPIReader,
		EnableIPv4:         true,
		EnableIPv6:         true,
		EnableSpiderSubnet: true,
	}
})
