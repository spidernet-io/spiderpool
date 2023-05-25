// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_underlay_one_test

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apitypes "k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MacvlanUnderlayOne", Label("underlay", "one-interface", "coordinator"), func() {

	It("spiderdoctor connectivity should be succeed", Label("C00001"), func() {
		// create task spiderdoctor crd
		task.Name = name
		// schedule
		plan.StartAfterMinute = 0
		plan.RoundNumber = 1
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
						Expect(v.Status).To(Equal("succeed"), "round %d failed", v.RoundNumber)
					}
					run = false
				}
				time.Sleep(time.Second * 5)
			}
		}
	})
})
