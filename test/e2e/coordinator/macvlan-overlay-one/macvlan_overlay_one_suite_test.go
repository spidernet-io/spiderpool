// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_overlay_one_test

import (
	"testing"

	multus_v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderdoctorV1 "github.com/spidernet-io/spiderdoctor/pkg/k8s/apis/spiderdoctor.spidernet.io/v1"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestMacvlanOverlayOne(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MacvlanOverlayOne Suite")
}

var frame *e2e.Framework

// var name string
var spiderDoctorAgent *appsv1.DaemonSet
var annotations = make(map[string]string)

var successRate = float64(1)
var name string
var err error
var delayMs = int64(15000)
var (
	task        *spiderdoctorV1.Nethttp
	plan        *spiderdoctorV1.SchedulePlan
	target      *spiderdoctorV1.NethttpTarget
	targetAgent *spiderdoctorV1.TargetAgentSepc
	request     *spiderdoctorV1.NethttpRequest
	condition   *spiderdoctorV1.NetSuccessCondition
	run         = true
)

var _ = BeforeSuite(func() {
	frame, err = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{multus_v1.AddToScheme, spiderpool.AddToScheme, spiderdoctorV1.AddToScheme})
	Expect(err).NotTo(HaveOccurred())

	if !common.CheckRunOverlayCNI() {
		Skip("overlay CNI is not installed , ignore this suite")
	}

})
