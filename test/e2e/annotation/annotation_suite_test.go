// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package annotation_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestAnnotation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Annotation Suite")
}

var frame *e2e.Framework
var ClusterDefaultV4IppoolList, ClusterDefaultV6IppoolList []string

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	frame, e = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{spiderpool.AddToScheme})
	Expect(e).NotTo(HaveOccurred())

	ClusterDefaultV4IppoolList, ClusterDefaultV6IppoolList, e = common.GetClusterDefaultIppool(frame)
	Expect(e).NotTo(HaveOccurred())
	if frame.Info.IpV4Enabled && len(ClusterDefaultV4IppoolList) == 0 {
		Fail("failed to find cluster ipv4 ippool")
	}
	if frame.Info.IpV6Enabled && len(ClusterDefaultV6IppoolList) == 0 {
		Fail("failed to find cluster ipv6 ippool")
	}
})
