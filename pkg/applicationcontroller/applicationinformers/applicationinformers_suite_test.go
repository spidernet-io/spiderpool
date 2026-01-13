// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package applicationinformers

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/spidernet-io/spiderpool/pkg/constant"
)

func TestApplicationinformers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Applicationinformers Suite")
}

var fakeReconcileFunc AppInformersAddOrUpdateFunc = func(ctx context.Context, oldObj, newObj interface{}) error {
	return constant.ErrUnknown
}

var fakeCleanupFunc APPInformersDelFunc = func(ctx context.Context, obj interface{}) error {
	return constant.ErrUnknown
}

var factory kubeinformers.SharedInformerFactory

var _ = BeforeSuite(func() {
	clientSet := fake.NewSimpleClientset()
	factory = kubeinformers.NewSharedInformerFactory(clientSet, 0)
})
