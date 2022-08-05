// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package nodemanager_test

import (
	"context"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager/mocks"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/nodemanager"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var nodeManager nodemanager.NodeManager
var ctx context.Context
var cancel context.CancelFunc
var err error
var k8sClient client.Client
var scheme *runtime.Scheme

func TestNodemanager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nodemanager Suite")
}

var _ = BeforeSuite(func() {

	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	scheme = runtime.NewScheme()
	err = corev1.AddToScheme(scheme)
	Expect(err).NotTo(HaveOccurred())

	// Mock Manager
	mgr := &mocks.Manager{}

	// Create a fake client
	k8sClient = createFakeClient()
	// Define the GetClient method
	mgr.On("GetClient").Return(k8sClient).Maybe()

	nodeManager, err = nodemanager.NewNodeManager(k8sClient)
	Expect(err).NotTo(HaveOccurred())
	Expect(nodeManager).NotTo(BeNil())

})

func createFakeClient() client.Client {
	return fakeclient.NewClientBuilder().
		WithScheme(scheme).
		Build()
}
