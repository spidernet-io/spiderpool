// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_underlay_one_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	kdoctorV1beta1 "github.com/kdoctor-io/kdoctor/pkg/k8s/apis/kdoctor.io/v1beta1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	apitypes "k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MacvlanUnderlayOne", Serial, Label("underlay", "one-interface", "coordinator"), func() {

	BeforeEach(func() {
		defer GinkgoRecover()
		// var e error
		task = new(kdoctorV1beta1.NetReach)
		targetAgent = new(kdoctorV1beta1.NetReachTarget)
		request = new(kdoctorV1beta1.NetHttpRequest)
		netreach = new(kdoctorV1beta1.AgentSpec)
		schedule = new(kdoctorV1beta1.SchedulePlan)
		condition = new(kdoctorV1beta1.NetSuccessCondition)

		name = "one-macvlan-standalone-" + tools.RandomName()

		// get macvlan-standalone multus crd instance by name
		multusInstance, err := frame.GetMultusInstance(common.MacvlanUnderlayVlan0, common.MultusNs)
		Expect(err).NotTo(HaveOccurred())
		Expect(multusInstance).NotTo(BeNil())

		// Update netreach.agentSpec to generate test Pods using the macvlan
		annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)
		netreach.Annotation = annotations
		netreach.HostNetwork = false
		GinkgoWriter.Printf("update kdoctoragent annotation: %v/%v annotation: %v \n", common.KDoctorAgentNs, common.KDoctorAgentDSName, annotations)
		task.Spec.AgentSpec = netreach
	})

	It("kdoctor connectivity should be succeed", Label("C00001"), Label("ebpf"), func() {

		enable := true
		disable := false
		// create task kdoctor crd
		task.Name = name
		GinkgoWriter.Printf("Start the netreach task: %v", task.Name)

		// Schedule
		crontab := "0 1"
		schedule.Schedule = &crontab
		schedule.RoundNumber = 1
		schedule.RoundTimeoutMinute = 1
		task.Spec.Schedule = schedule

		// target
		targetAgent.Ingress = &disable
		targetAgent.Endpoint = &enable
		targetAgent.ClusterIP = &enable
		targetAgent.MultusInterface = &disable
		targetAgent.NodePort = &enable
		targetAgent.IPv4 = &frame.Info.IpV4Enabled
		targetAgent.IPv6 = &frame.Info.IpV6Enabled
		targetAgent.EnableLatencyMetric = true
		GinkgoWriter.Printf("targetAgent for kdoctor %+v", targetAgent)
		task.Spec.Target = targetAgent

		// request
		request.DurationInSecond = 10
		request.QPS = 3
		request.PerRequestTimeoutInMS = 15000
		task.Spec.Request = request

		// success condition
		condition.SuccessRate = &successRate
		condition.MeanAccessDelayInMs = &delayMs
		task.Spec.SuccessCondition = condition
		taskCopy := task

		GinkgoWriter.Printf("kdoctor task: %+v \n", task)
		err := frame.CreateResource(task)
		Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd create failed")

		err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
		Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd get failed")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second*60*5)
		defer cancel()

		var err1 = errors.New("error has occurred")

		for run {
			select {
			case <-ctx.Done():
				run = false
				Expect(errors.New("wait nethttp test timeout")).NotTo(HaveOccurred(), " running kdoctor task timeout")
			default:
				err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
				Expect(err).NotTo(HaveOccurred(), " kdoctor nethttp crd get failed")

				if taskCopy.Status.Finish == true {
					command := fmt.Sprintf("get netreaches.kdoctor.io %s -oyaml", taskCopy.Name)
					netreachesLog, _ := frame.ExecKubectl(command, ctx)
					GinkgoWriter.Printf("kdoctor's netreaches execution result %+v \n", string(netreachesLog))

					for _, v := range taskCopy.Status.History {
						if v.Status == "succeed" {
							err1 = nil
						}
					}
					run = false

					ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*30)
					defer cancel1()
					for {
						select {
						case <-ctx1.Done():
							Expect(errors.New("wait kdoctorreport timeout")).NotTo(HaveOccurred(), "failed to run kdoctor task and wait kdoctorreport timeout")
						default:
							command = fmt.Sprintf("get kdoctorreport %s -oyaml", taskCopy.Name)
							kdoctorreportLog, err := frame.ExecKubectl(command, ctx)
							if err != nil {
								time.Sleep(common.ForcedWaitingTime)
								continue
							}
							GinkgoWriter.Printf("kdoctor's kdoctorreport execution result %+v \n", string(kdoctorreportLog))
						}
						break
					}
				}
				time.Sleep(time.Second * 5)
			}
		}
		Expect(err1).NotTo(HaveOccurred())
	})
})
