// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_overlay_one_test

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("MacvlanOverlayOne", Label("overlay", "one-nic", "coordinator"), func() {

	It("spiderdoctor connectivity should be succeed", Label("C00002"), func() {
		// create task spiderdoctor crd
		//	task.Name = name
		//	// schedule
		//	plan.StartAfterMinute = 0
		//	plan.RoundNumber = 2
		//	plan.IntervalMinute = 2
		//	plan.TimeoutMinute = 2
		//	task.Spec.Schedule = plan
		//	// target
		//	targetAgent.TestIngress = false
		//	targetAgent.TestEndpoint = true
		//	targetAgent.TestClusterIp = true
		//	targetAgent.TestMultusInterface = frame.Info.MultusEnabled
		//	targetAgent.TestNodePort = true
		//	targetAgent.TestIPv4 = &frame.Info.IpV4Enabled
		//	targetAgent.TestIPv6 = &frame.Info.IpV6Enabled
		//
		//	target.TargetAgent = targetAgent
		//	task.Spec.Target = target
		//	// request
		//	request.DurationInSecond = 5
		//	request.QPS = 1
		//	request.PerRequestTimeoutInMS = 15000
		//
		//	task.Spec.Request = request
		//	// success condition
		//
		//	condition.SuccessRate = &successRate
		//	condition.MeanAccessDelayInMs = &delayMs
		//
		//	task.Spec.SuccessCondition = condition
		//
		//	taskCopy := task
		//	GinkgoWriter.Printf("spiderdoctor task: %+v", task)
		//	err := frame.CreateResource(task)
		//	Expect(err).NotTo(HaveOccurred(), " spiderdoctor nethttp crd create failed")
		//
		//	err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
		//	Expect(err).NotTo(HaveOccurred(), " spiderdoctor nethttp crd get failed")
		//	ctx, cancel := context.WithTimeout(context.Background(), time.Second*60*10)
		//	defer cancel()
		//
		//	var err1 = errors.New("error has occurred")
		//
		//	for run {
		//		select {
		//		case <-ctx.Done():
		//			run = false
		//			Expect(errors.New("wait nethttp test timeout")).NotTo(HaveOccurred(), " running spiderdoctor task timeout")
		//		default:
		//			err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
		//			Expect(err).NotTo(HaveOccurred(), " spiderdoctor nethttp crd get failed")
		//
		//			if taskCopy.Status.Finish == true {
		//				for _, v := range taskCopy.Status.History {
		//					if v.Status == "succeed" {
		//						err1 = nil
		//					}
		//				}
		//				run = false
		//			}
		//			time.Sleep(time.Second * 5)
		//		}
		//	}
		//	Expect(err1).NotTo(HaveOccurred())
	})
})
