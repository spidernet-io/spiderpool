// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
)

var scheme *runtime.Scheme
var fakeClient client.Client
var tracker k8stesting.ObjectTracker
var fakeAPIReader client.Reader
var rIPManager reservedipmanager.ReservedIPManager
var rIPWebhook *reservedipmanager.ReservedIPWebhook

func TestReservedIPManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ReservedIPManager Suite", Label("reservedipmanager", "unittest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := spiderpoolv2beta1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&spiderpoolv2beta1.SpiderReservedIP{}, metav1.ObjectNameField, func(raw client.Object) []string {
			rIP := raw.(*spiderpoolv2beta1.SpiderReservedIP)
			return []string{rIP.GetObjectMeta().GetName()}
		}).
		WithIndex(&spiderpoolv2beta1.SpiderReservedIP{}, constant.SpecIPVersionField, func(raw client.Object) []string {
			rIP := raw.(*spiderpoolv2beta1.SpiderReservedIP)
			return []string{strconv.FormatInt(*rIP.Spec.IPVersion, 10)}
		}).
		Build()

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&spiderpoolv2beta1.SpiderReservedIP{}, metav1.ObjectNameField, func(raw client.Object) []string {
			rIP := raw.(*spiderpoolv2beta1.SpiderReservedIP)
			return []string{rIP.GetObjectMeta().GetName()}
		}).
		WithIndex(&spiderpoolv2beta1.SpiderReservedIP{}, constant.SpecIPVersionField, func(raw client.Object) []string {
			rIP := raw.(*spiderpoolv2beta1.SpiderReservedIP)
			return []string{strconv.FormatInt(*rIP.Spec.IPVersion, 10)}
		}).
		Build()

	rIPManager, err = reservedipmanager.NewReservedIPManager(
		fakeClient,
		fakeAPIReader,
	)
	Expect(err).NotTo(HaveOccurred())

	rIPWebhook = &reservedipmanager.ReservedIPWebhook{}
})
