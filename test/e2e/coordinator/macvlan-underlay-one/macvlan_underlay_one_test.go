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
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
)

var _ = Describe("MacvlanUnderlayOne", Serial, Label("underlay", "one-interface", "coordinator"), func() {

	BeforeEach(func() {
		defer GinkgoRecover()
		var e error
		task = new(kdoctorV1beta1.NetReach)
		targetAgent = new(kdoctorV1beta1.NetReachTarget)
		request = new(kdoctorV1beta1.NetHttpRequest)
		schedule = new(kdoctorV1beta1.SchedulePlan)
		condition = new(kdoctorV1beta1.NetSuccessCondition)

		name = "one-macvlan-standalone-" + tools.RandomName()

		// get macvlan-standalone multus crd instance by name
		multusInstance, err := frame.GetMultusInstance(common.MacvlanUnderlayVlan0, common.MultusNs)
		Expect(err).NotTo(HaveOccurred())
		Expect(multusInstance).NotTo(BeNil())

		annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", common.MultusNs, common.MacvlanUnderlayVlan0)

		GinkgoWriter.Printf("update kdoctoragent annotation: %v/%v annotation: %v \n", common.KDoctorAgentNs, common.KDoctorAgentDSName, annotations)
		kdoctorAgent, e = frame.GetDaemonSet(common.KDoctorAgentDSName, common.KDoctorAgentNs)
		Expect(e).NotTo(HaveOccurred())
		Expect(kdoctorAgent).NotTo(BeNil())

		err = frame.DeleteDaemonSet(common.KDoctorAgentDSName, common.KDoctorAgentNs)
		Expect(err).NotTo(HaveOccurred())

		time.Sleep(10 * time.Second)

		// issue: the object has been modified; please apply your changes to the latest version and try again
		kdoctorAgent.ResourceVersion = ""
		kdoctorAgent.CreationTimestamp = v1.Time{}
		kdoctorAgent.UID = apitypes.UID("")
		kdoctorAgent.Spec.Template.Annotations = annotations

		err = frame.CreateDaemonSet(kdoctorAgent)
		Expect(err).NotTo(HaveOccurred())

		nodeList, err := frame.GetNodeList()
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()

		err = frame.WaitPodListRunning(kdoctorAgent.Spec.Selector.MatchLabels, len(nodeList.Items), ctx)
		Expect(err).NotTo(HaveOccurred())

		// Updated network configuration for kdoctor pod. After the pod is fully running,
		// we still need to wait for a certain amount of time for the routing to complete synchronization.
		// Otherwise kdoctor may fail to detect it.
		time.Sleep(20 * time.Second)
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
		targetAgent.MultusInterface = &enable
		targetAgent.NodePort = &enable
		targetAgent.IPv4 = &frame.Info.IpV4Enabled
		targetAgent.IPv6 = &frame.Info.IpV6Enabled
		targetAgent.EnableLatencyMetric = true
		GinkgoWriter.Printf("targetAgent for kdoctor %+v", targetAgent)
		task.Spec.Target = targetAgent

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

		GinkgoWriter.Printf("kdoctor task: %+v", task)
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
