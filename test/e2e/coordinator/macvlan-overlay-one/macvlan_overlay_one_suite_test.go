// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_overlay_one_test

import (
	"fmt"
	"testing"

	multus_v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	kdoctorV1beta1 "github.com/kdoctor-io/kdoctor/pkg/k8s/apis/kdoctor.io/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

func TestMacvlanOverlayOne(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MacvlanOverlayOne Suite")
}

var frame *e2e.Framework

// var name string
var successRate = float64(1)
var name string
var err error
var delayMs = int64(15000)
var (
	task           *kdoctorV1beta1.NetReach
	netreach       *kdoctorV1beta1.AgentSpec
	targetAgent    *kdoctorV1beta1.NetReachTarget
	request        *kdoctorV1beta1.NetHttpRequest
	schedule       *kdoctorV1beta1.SchedulePlan
	condition      *kdoctorV1beta1.NetSuccessCondition
	run            = true
	macvlanVlan0   = "test-macvlan1"
	macvlanVlan100 = "test-macvlan2"
	macvlanVlan200 = "test-macvlan3"
)

var _ = BeforeSuite(func() {
	frame, err = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{multus_v1.AddToScheme, spiderpool.AddToScheme, kdoctorV1beta1.AddToScheme})
	Expect(err).NotTo(HaveOccurred())

	if !common.CheckRunOverlayCNI() {
		Skip("overlay CNI is not installed , ignore this suite")
	}

	// create spidermultusconfig to set rp_filter to 1, which can
	// better to test the connection
	for idx, name := range []string{common.MacvlanUnderlayVlan0, common.MacvlanVlan100, common.MacvlanVlan200} {
		smc, err := frame.GetSpiderMultusInstance(common.MultusNs, name)
		Expect(err).NotTo(HaveOccurred())

		copy := smc.Spec
		copy.CoordinatorConfig.HostRPFilter = ptr.To(1)
		err = frame.CreateSpiderMultusInstance(&spiderpool.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("test-macvlan%d", idx+1),
				Namespace: common.MultusNs,
			},
			Spec: copy,
		})

		if err == nil || apierrors.IsAlreadyExists(err) {
			GinkgoWriter.Printf("create spiderMultusConfig %s/%s successfully\n", common.MultusNs, fmt.Sprintf("test-macvlan%d", idx+1))
			continue
		}

		Expect(err).NotTo(HaveOccurred())
	}
})

// var _ = AfterSuite(func() {
// delete the test spidermultusconfig
//for idx := range []string{common.MacvlanUnderlayVlan0, common.MacvlanVlan100, common.MacvlanVlan200} {
//	err = frame.DeleteSpiderMultusInstance(common.MultusNs, fmt.Sprintf("test-macvlan%d", idx+1))
//	GinkgoWriter.Printf("failed to delete spiderMultusConfig: %v\n", err)
// Expect(err).NotTo(HaveOccurred())
//}
//})
