// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_underlay_one_test

import (
	"testing"

	multus_v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	kdoctorV1beta1 "github.com/kdoctor-io/kdoctor/pkg/k8s/apis/kdoctor.io/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestMacvlanStandaloneOne(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MacvlanStandaloneOne Suite")
}

var (
	frame       *e2e.Framework
	name        string
	err         error
	annotations = make(map[string]string)
	successRate = float64(1)
	delayMs     = int64(15000)
)

var (
	task        *kdoctorV1beta1.NetReach
	netreach    *kdoctorV1beta1.AgentSpec
	targetAgent *kdoctorV1beta1.NetReachTarget
	request     *kdoctorV1beta1.NetHttpRequest
	condition   *kdoctorV1beta1.NetSuccessCondition
	schedule    *kdoctorV1beta1.SchedulePlan
	run         = true
)

var _ = BeforeSuite(func() {
	frame, err = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{multus_v1.AddToScheme, spiderpool.AddToScheme, kdoctorV1beta1.AddToScheme})
	Expect(err).NotTo(HaveOccurred())

	if common.CheckRunOverlayCNI() {
		Skip("overlay CNI is installed , ignore this suite")
	}
})
