// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_underlay_one_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/spidernet-io/spiderpool/test/e2e/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	spiderdoctorV1 "github.com/spidernet-io/spiderdoctor/pkg/k8s/apis/spiderdoctor.spidernet.io/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MacvlanUnderlayOne", Serial, Label("underlay", "one-interface", "coordinator"), func() {

	BeforeEach(func() {
		defer GinkgoRecover()
		var e error
		task = new(spiderdoctorV1.Nethttp)
		plan = new(spiderdoctorV1.SchedulePlan)
		target = new(spiderdoctorV1.NethttpTarget)
		targetAgent = new(spiderdoctorV1.TargetAgentSepc)
		request = new(spiderdoctorV1.NethttpRequest)
		condition = new(spiderdoctorV1.NetSuccessCondition)

		name = "one-macvlan-standalone-" + tools.RandomName()

		// get macvlan-standalone multus crd instance by name
		multusInstance, err := frame.GetMultusInstance(common.MacvlanUnderlayVlan0, common.MultusNs)
		Expect(err).NotTo(HaveOccurred())
		Expect(multusInstance).NotTo(BeNil())

		annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)

		GinkgoWriter.Printf("update spiderdoctoragent annotation: %v/%v annotation: %v \n", common.SpiderDoctorAgentNs, common.SpiderDoctorAgentDSName, annotations)
		spiderDoctorAgent, e = frame.GetDaemonSet(common.SpiderDoctorAgentDSName, common.SpiderDoctorAgentNs)
		Expect(e).NotTo(HaveOccurred())
		Expect(spiderDoctorAgent).NotTo(BeNil())

		err = frame.DeleteDaemonSet(common.SpiderDoctorAgentDSName, common.SpiderDoctorAgentNs)
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(10 * time.Second)

		// issue: the object has been modified; please apply your changes to the latest version and try again
		spiderDoctorAgent.ResourceVersion = ""
		spiderDoctorAgent.CreationTimestamp = v1.Time{}
		spiderDoctorAgent.UID = apitypes.UID("")
		spiderDoctorAgent.Spec.Template.Annotations = annotations

		err = frame.CreateDaemonSet(spiderDoctorAgent)
		Expect(err).NotTo(HaveOccurred())

		nodeList, err := frame.GetNodeList()
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()

		err = frame.WaitPodListRunning(spiderDoctorAgent.Spec.Selector.MatchLabels, len(nodeList.Items), ctx)
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(20 * time.Second)
	})

	It("spiderdoctor connectivity should be succeed", Label("C00001"), func() {
		// create task spiderdoctor crd
		task.Name = name
		// schedule
		plan.StartAfterMinute = 0
		plan.RoundNumber = 2
		plan.IntervalMinute = 2
		plan.TimeoutMinute = 2
		task.Spec.Schedule = plan
		// target
		targetAgent.TestIngress = false
		targetAgent.TestEndpoint = true
		targetAgent.TestClusterIp = true
		targetAgent.TestMultusInterface = true
		targetAgent.TestNodePort = true
		targetAgent.TestIPv4 = &frame.Info.IpV4Enabled
		targetAgent.TestIPv6 = &frame.Info.IpV6Enabled

		target.TargetAgent = targetAgent
		task.Spec.Target = target
		// request
		request.DurationInSecond = 5
		request.QPS = 1
		request.PerRequestTimeoutInMS = 15000

		task.Spec.Request = request
		// success condition

		condition.SuccessRate = &successRate
		condition.MeanAccessDelayInMs = &delayMs

		task.Spec.SuccessCondition = condition
		taskCopy := task

		GinkgoWriter.Printf("spiderdoctor task: %+v", task)
		err := frame.CreateResource(task)
		Expect(err).NotTo(HaveOccurred(), " spiderdoctor nethttp crd create failed")

		err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
		Expect(err).NotTo(HaveOccurred(), " spiderdoctor nethttp crd get failed")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60*5)
		defer cancel()

		var err1 = errors.New("error has occurred")

		for run {
			select {
			case <-ctx.Done():
				run = false
				Expect(errors.New("wait nethttp test timeout")).NotTo(HaveOccurred(), " running spiderdoctor task timeout")
			default:
				err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
				Expect(err).NotTo(HaveOccurred(), " spiderdoctor nethttp crd get failed")
				if taskCopy.Status.Finish == true {
					for _, v := range taskCopy.Status.History {
						if v.Status == "succeed" {
							err1 = nil
						}
					}
					run = false
				}
				time.Sleep(time.Second * 5)
			}
		}
		Expect(err1).NotTo(HaveOccurred())
	})
})
