// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	admissionv1 "k8s.io/api/admission/v1"
)

var err error
var cfg *rest.Config
var k8sClient client.Client
var testenv *envtest.Environment
var rIPManager reservedipmanager.ReservedIPManager
var mgr manager.Manager

var ctx, cancel = context.WithCancel(context.TODO())

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Cmd Suite")
}

var _ = BeforeSuite(func() {
	testenv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "..", "charts", "spiderpool", "crds")},
		ErrorIfCRDPathMissing: false,
	}
	cfg, err = testenv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	scheme = runtime.NewScheme()
	err = spiderpoolv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	err = admissionv1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	k8sClient, err = client.New(cfg, client.Options{
		Scheme: scheme,
	})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err = manager.New(cfg, manager.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
	Expect(err).ShouldNot(HaveOccurred())

	rIPManager, err = reservedipmanager.NewReservedIPManager(mgr.GetClient(), mgr)
	Expect(err).ShouldNot(HaveOccurred())
	Expect(rIPManager).NotTo(BeNil())

	go func() {
		defer GinkgoRecover()

		err = mgr.Start(ctx)
		Expect(err).NotTo(HaveOccurred())
	}()

})

var _ = AfterSuite(func() {
	cancel()
	err = testenv.Stop()
	Expect(err).NotTo(HaveOccurred())

})
