// Copyright 2023 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package macvlan_overlay_one_test

import (
	"context"
	"errors"
	"fmt"
	"time"

	spiderdoctorV1 "github.com/spidernet-io/spiderdoctor/pkg/k8s/apis/spiderdoctor.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MacvlanOverlayOne", Serial, Label("overlay", "one-nic", "coordinator"), func() {

	Context("Macvlan Overlay One for Calico", Label("calico"), func() {

		BeforeEach(func() {
			defer GinkgoRecover()
			task = new(spiderdoctorV1.Nethttp)
			plan = new(spiderdoctorV1.SchedulePlan)
			target = new(spiderdoctorV1.NethttpTarget)
			targetAgent = new(spiderdoctorV1.TargetAgentSepc)
			request = new(spiderdoctorV1.NethttpRequest)
			condition = new(spiderdoctorV1.NetSuccessCondition)

			name = "one-macvlan-overlay-" + tools.RandomName()

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

		It("spiderdoctor connectivity should be succeed", Label("C00002"), func() {
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
			targetAgent.TestMultusInterface = frame.Info.MultusEnabled
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
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60*10)
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

	Context("Macvlan Overlay One for Cilium", Label("Cilium"), func() {

		BeforeEach(func() {
			defer GinkgoRecover()
			task = new(spiderdoctorV1.Nethttp)
			plan = new(spiderdoctorV1.SchedulePlan)
			target = new(spiderdoctorV1.NethttpTarget)
			targetAgent = new(spiderdoctorV1.TargetAgentSepc)
			request = new(spiderdoctorV1.NethttpRequest)
			condition = new(spiderdoctorV1.NetSuccessCondition)

			name = "one-macvlan-overlay-" + tools.RandomName()

			annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.CiliumCNIName)
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

			// debug error
			// ctx, cancel = context.WithTimeout(context.Background(), common.PodStartTimeout)
			// defer cancel()
			// podList, err := frame.GetPodListByLabel(spiderDoctorAgent.Spec.Selector.MatchLabels)
			// Expect(err).NotTo(HaveOccurred())

			// for _, v := range podList.Items {
			// 	var commandString string
			// 	if !frame.Info.IpV4Enabled && frame.Info.IpV6Enabled {
			// 		commandString = fmt.Sprintf("ping -6 %v -c 2", v.Status.PodIP)
			// 	} else {
			// 		commandString = fmt.Sprintf("ping %v -c 2", v.Status.PodIP)
			// 	}
			// 	out, err := frame.ExecCommandInPod(commandString, v.Name, v.Namespace, ctx)
			// 	GinkgoWriter.Printf("xxxxxxxxxxxx %v", string(out))
			// 	Expect(err).NotTo(HaveOccurred(), "err is xxxxx: %v", err)
			// }
			time.Sleep(20 * time.Second)
		})

		It("spiderdoctor connectivity should be succeed", Label("C00002"), func() {
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
			targetAgent.TestMultusInterface = frame.Info.MultusEnabled
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
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*60*10)
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
						GinkgoWriter.Printf("test result %+v \n", taskCopy)
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
})
