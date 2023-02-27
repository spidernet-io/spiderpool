// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Controllers Suite")
}

var factory kubeinformers.SharedInformerFactory

var _ = BeforeSuite(func() {
	clientSet := fake.NewSimpleClientset()
	factory = kubeinformers.NewSharedInformerFactory(clientSet, 0)
})
