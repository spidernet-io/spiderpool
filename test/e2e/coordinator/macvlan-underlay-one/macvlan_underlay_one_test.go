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
	corev1 "k8s.io/api/core/v1"
	api_errors "k8s.io/apimachinery/pkg/api/errors"
	apitypes "k8s.io/apimachinery/pkg/types"

	"github.com/spidernet-io/spiderpool/test/e2e/common"
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

	It("kdoctor connectivity should be succeed", Label("C00001", "C00013"), Label("ebpf"), func() {

		enable := true
		disable := false
		// create task kdoctor crd
		task.Name = name
		GinkgoWriter.Printf("Start the netreach task: %v", task.Name)

		// Schedule
		crontab := "1 1"
		schedule.Schedule = &crontab
		// The sporadic test failures in kdoctor were attempted to be reproduced, but couldn't be.
		// By leveraging kdoctor's loop testing, if a failure occurs in the first test,
		// check whether it also fails on the second attempt.
		schedule.RoundNumber = 3
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
		request.QPS = 1
		request.PerRequestTimeoutInMS = 7000
		task.Spec.Request = request

		// success condition
		condition.SuccessRate = &successRate
		condition.MeanAccessDelayInMs = &delayMs
		task.Spec.SuccessCondition = condition

		err := frame.CreateResource(task)
		Expect(err).NotTo(HaveOccurred(), "failed to create kdoctor task")
		GinkgoWriter.Printf("succeeded to create kdoctor task: %+v \n", task)

		// update the kdoctor service to use corev1.ServiceExternalTrafficPolicyLocal
		if frame.Info.IpV4Enabled {
			kdoctorIPv4ServiceName := fmt.Sprintf("%s-%s-ipv4", "kdoctor-netreach", task.Name)
			var kdoctorIPv4Service *corev1.Service
			Eventually(func() bool {
				kdoctorIPv4Service, err = frame.GetService(kdoctorIPv4ServiceName, "kube-system")
				if api_errors.IsNotFound(err) {
					return false
				}
				if err != nil {
					return false
				}
				return true
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
			kdoctorIPv4Service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
			kdoctorIPv4Service.Spec.Type = corev1.ServiceTypeNodePort
			Expect(frame.UpdateResource(kdoctorIPv4Service)).NotTo(HaveOccurred())
		}
		if frame.Info.IpV6Enabled {
			kdoctorIPv6ServiceName := fmt.Sprintf("%s-%s-ipv6", "kdoctor-netreach", task.Name)
			var kdoctorIPv6Service *corev1.Service
			Eventually(func() bool {
				kdoctorIPv6Service, err = frame.GetService(kdoctorIPv6ServiceName, "kube-system")
				if api_errors.IsNotFound(err) {
					return false
				}
				if err != nil {
					return false
				}
				return true
			}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
			kdoctorIPv6Service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyLocal
			kdoctorIPv6Service.Spec.Type = corev1.ServiceTypeNodePort
			Expect(frame.UpdateResource(kdoctorIPv6Service)).NotTo(HaveOccurred())
		}

		// waiting for kdoctor task to finish
		ctx, cancel := context.WithTimeout(context.Background(), common.KDoctorRunTimeout)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				Expect(errors.New("timeout waiting for kdoctor task to finish")).NotTo(HaveOccurred())
			default:
				taskCopy := task
				err = frame.GetResource(apitypes.NamespacedName{Name: name}, taskCopy)
				Expect(err).NotTo(HaveOccurred(), "Failed to get kdoctor task")
				if taskCopy.Status.Finish {
					roundFailed := false
					for _, t := range taskCopy.Status.History {
						// No configuration has been changed, The first round of the test is not considered a failure
						if t.RoundNumber != 1 && t.Status == "failed" {
							roundFailed = true
							break
						}
					}
					if roundFailed {
						Fail("kdoctor task is not successful")
					}
					return
				}
				for _, t := range taskCopy.Status.History {
					// If the check is successful, exit directly.
					if t.RoundNumber == 1 && t.Status == "succeed" {
						GinkgoWriter.Println("succeed to run kdoctor task")
						return
					}
					// If the check fails, we should collect the failed Pod network information as soon as possible
					// If the first attempt failed but the second attempt succeeded,
					// we collected network logs and compared the two attempts to see if there were any differences.
					if t.Status == "failed" || (t.RoundNumber != 1 && t.Status == "succeed") {
						GinkgoLogr.Error(fmt.Errorf("Failed to run kdoctor task, round %d, at time %s", t.RoundNumber, time.Now()), "Failed")
						podList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/name": taskCopy.Name})
						Expect(err).NotTo(HaveOccurred(), "Failed to get pod list by label")
						Expect(common.GetPodNetworkInfo(ctx, frame, podList)).NotTo(HaveOccurred(), "Failed to get pod network info")
						Expect(common.GetNodeNetworkInfo(ctx, frame, frame.Info.KindNodeList)).NotTo(HaveOccurred(), "Failed to get node network info")
					}
				}
			}
		}
	})
})
