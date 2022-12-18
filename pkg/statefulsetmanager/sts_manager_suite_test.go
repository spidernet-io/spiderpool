// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package statefulsetmanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/pkg/statefulsetmanager"
)

var scheme *runtime.Scheme
var fakeClient client.Client
var stsManager statefulsetmanager.StatefulSetManager

func TestStatefulSetManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "StatefulSetManager Suite", Label("statefulsetmanager", "unitest"))
}

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := appsv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	stsManager, err = statefulsetmanager.NewStatefulSetManager(fakeClient)
	Expect(err).NotTo(HaveOccurred())
})
