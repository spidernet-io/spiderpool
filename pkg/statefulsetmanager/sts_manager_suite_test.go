// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package statefulsetmanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
)

var (
	scheme        *runtime.Scheme
	fakeClient    client.Client
	tracker       k8stesting.ObjectTracker
	fakeAPIReader client.Reader
	stsManager    statefulsetmanager.StatefulSetManager
)

func TestStatefulSetManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "StatefulSetManager Suite", Label("statefulsetmanager", "unittest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := appsv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&appsv1.StatefulSet{}, metav1.ObjectNameField, func(raw client.Object) []string {
			sts := raw.(*appsv1.StatefulSet)
			return []string{sts.GetObjectMeta().GetName()}
		}).
		Build()

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&appsv1.StatefulSet{}, metav1.ObjectNameField, func(raw client.Object) []string {
			sts := raw.(*appsv1.StatefulSet)
			return []string{sts.GetObjectMeta().GetName()}
		}).
		Build()

	stsManager, err = statefulsetmanager.NewStatefulSetManager(
		fakeClient,
		fakeAPIReader,
	)
	Expect(err).NotTo(HaveOccurred())
})
