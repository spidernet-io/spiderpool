// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
	k8stesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
)

var (
	scheme        *runtime.Scheme
	fakeClient    client.Client
	tracker       k8stesting.ObjectTracker
	fakeAPIReader client.Reader
	nodeManager   nodemanager.NodeManager
)

func TestNodeManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NodeManager Suite", Label("nodemanager", "unittest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		WithIndex(&corev1.Node{}, metav1.ObjectNameField, func(raw client.Object) []string {
			node := raw.(*corev1.Node)
			return []string{node.GetObjectMeta().GetName()}
		}).
		Build()

	tracker = k8stesting.NewObjectTracker(scheme, k8sscheme.Codecs.UniversalDecoder())
	fakeAPIReader = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjectTracker(tracker).
		WithIndex(&corev1.Node{}, metav1.ObjectNameField, func(raw client.Object) []string {
			node := raw.(*corev1.Node)
			return []string{node.GetObjectMeta().GetName()}
		}).
		Build()

	nodeManager, err = nodemanager.NewNodeManager(
		fakeClient,
		fakeAPIReader,
	)
	Expect(err).NotTo(HaveOccurred())
})
