// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package namespacemanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
)

var scheme *runtime.Scheme
var fakeClient client.Client
var nsManager namespacemanager.NamespaceManager

func TestNamespaceManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "NamespaceManager Suite", Label("namespacemanager", "unitest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	nsManager, err = namespacemanager.NewNamespaceManager(fakeClient)
	Expect(err).NotTo(HaveOccurred())
})
