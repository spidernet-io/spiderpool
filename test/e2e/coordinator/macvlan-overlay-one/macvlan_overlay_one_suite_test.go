// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_overlay_one_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"

	multus_v1 "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	e2e "github.com/spidernet-io/e2eframework/framework"
	spiderdoctorV1 "github.com/spidernet-io/spiderdoctor/pkg/k8s/apis/spiderdoctor.spidernet.io/v1"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

func TestMacvlanOverlayOne(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "MacvlanOverlayOne Suite")
}

var frame *e2e.Framework

// var name string
var spiderDoctorAgent *appsv1.DaemonSet
var annotations = make(map[string]string)

//var successRate = float64(1)
//var delayMs = int64(15000)
//var (
//	task        *spiderdoctorV1.Nethttp
//	plan        *spiderdoctorV1.SchedulePlan
//	target      *spiderdoctorV1.NethttpTarget
//	targetAgent *spiderdoctorV1.TargetAgentSepc
//	request     *spiderdoctorV1.NethttpRequest
//	condition   *spiderdoctorV1.NetSuccessCondition
//	run         = true
//)

var _ = BeforeSuite(func() {
	defer GinkgoRecover()
	var e error
	//task = new(spiderdoctorV1.Nethttp)
	//plan = new(spiderdoctorV1.SchedulePlan)
	//target = new(spiderdoctorV1.NethttpTarget)
	//targetAgent = new(spiderdoctorV1.TargetAgentSepc)
	//request = new(spiderdoctorV1.NethttpRequest)
	//condition = new(spiderdoctorV1.NetSuccessCondition)

	frame, e = e2e.NewFramework(GinkgoT(), []func(*runtime.Scheme) error{multus_v1.AddToScheme, spiderpool.AddToScheme, spiderdoctorV1.AddToScheme})
	Expect(e).NotTo(HaveOccurred())

	//name = "one-macvlan-overlay-" + tools.RandomName()

	// get calico multus crd instance by name
	calicoMultusInstance, err := frame.GetMultusInstance(common.CalicoCNIName, common.MultusNs)
	Expect(err).NotTo(HaveOccurred())
	Expect(calicoMultusInstance).NotTo(BeNil())

	// get calico multus crd instance by name
	macvlanOverlay100MultusInstance, err := frame.GetMultusInstance(common.MacvlanOverlayVlan100, common.MultusNs)
	Expect(err).NotTo(HaveOccurred())
	Expect(macvlanOverlay100MultusInstance).NotTo(BeNil())

	annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.CalicoCNIName)
	annotations[common.MultusNetworks] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanOverlayVlan100)

	if frame.Info.IpV4Enabled && frame.Info.IpV6Enabled {
		annotations[common.SpiderPoolSubnetAnnotationKey] = `{"interface": "net1", "ipv4": ["vlan100-v4"], "ipv6": ["vlan100-v6"]}`
	} else if frame.Info.IpV4Enabled && !frame.Info.IpV6Enabled {
		annotations[common.SpiderPoolSubnetAnnotationKey] = `{"interface": "net1", "ipv4": ["vlan100-v4"]}`
	} else {
		annotations[common.SpiderPoolSubnetAnnotationKey] = `{"interface": "net1", "ipv6": ["vlan100-v6"]}`
	}

	GinkgoWriter.Printf("update spiderdoctoragent annotation: %v/%v annotation: %v \n", common.SpiderDoctorAgentNs, common.SpiderDoctorAgentDSName, annotations)
	spiderDoctorAgent, err = frame.GetDaemonSet(common.SpiderDoctorAgentDSName, common.SpiderDoctorAgentNs)
	Expect(err).NotTo(HaveOccurred())
	Expect(spiderDoctorAgent).NotTo(BeNil())

	// issue: the object has been modified; please apply your changes to the latest version and try again
	spiderDoctorAgent.ResourceVersion = ""
	spiderDoctorAgent.CreationTimestamp = v1.Time{}
	spiderDoctorAgent.UID = types.UID("")

	spiderDoctorAgent.Spec.Template.Annotations = annotations
	err = frame.UpdateResource(spiderDoctorAgent)
	Expect(err).NotTo(HaveOccurred())

	time.Sleep(30 * time.Second)

	nodeList, err := frame.GetNodeList()
	Expect(err).NotTo(HaveOccurred())

	ctx, cancel := context.WithTimeout(context.Background(), 3*common.PodStartTimeout)
	defer cancel()

	err = frame.WaitPodListRunning(spiderDoctorAgent.Spec.Selector.MatchLabels, len(nodeList.Items), ctx)
	Expect(err).NotTo(HaveOccurred())

	time.Sleep(30 * time.Second)
})
