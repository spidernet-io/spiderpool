// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
)

func TestReservedIPManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ReservedIPManager Suite", Label("reservedipmanager", "unitest"))
}

var scheme *runtime.Scheme
var fakeClient client.Client
var rIPManager reservedipmanager.ReservedIPManager

var _ = BeforeSuite(func() {
	scheme = runtime.NewScheme()
	err := spiderpoolv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	fakeClient = fake.NewClientBuilder().
		WithScheme(scheme).
		Build()

	rIPManager, err = reservedipmanager.NewReservedIPManager(fakeClient)
	Expect(err).NotTo(HaveOccurred())
})
