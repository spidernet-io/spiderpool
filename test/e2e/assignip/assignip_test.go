// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package assignip_test

import (
	"context"
	"time"

	"fmt"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test pod", Label("assignip"), func() {
	var err error
	var testName, namespace string
	var dpm *appsv1.Deployment
	var sts *appsv1.StatefulSet
	var ds *appsv1.DaemonSet
	var rs *appsv1.ReplicaSet
	var podlist *corev1.PodList

	BeforeEach(func() {
		// init namespace name and create
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespace(namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
		// init test pod name
		testName = "pod" + tools.RandomName()
		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		})
	})

	DescribeTable("test pod assign ip", func(annotationLength int) {
		// try to create pod
		GinkgoWriter.Printf("create pod %v/%v with annotationLength= %v \n", namespace, testName, annotationLength)
		podYaml := common.GenerateLongPodYaml(testName, namespace, annotationLength)
		Expect(podYaml).NotTo(BeNil())

		pod, _, _ := common.CreatePodUntilReady(frame, podYaml, testName, namespace, time.Second*30)
		Expect(pod).NotTo(BeNil())
		Expect(pod.Annotations["test"]).To(Equal(podYaml.Annotations["test"]))
		GinkgoWriter.Printf("create pod %v/%v successfully \n", namespace, testName)

		v := &corev1.PodList{
			Items: []corev1.Pod{*pod},
		}
		ok, e := common.CheckPodIpRecordInIppool(frame, ClusterDefaultV4IpoolList, ClusterDefaultV6IpoolList, v)
		if e != nil || !ok {
			Fail(fmt.Sprintf("failed to CheckPodIpRecordInIppool, reason=%v", e))
		}

		// try to delete pod
		GinkgoWriter.Printf("delete pod %v/%v \n", namespace, testName)
		err = frame.DeletePod(testName, namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", namespace, testName)
	},
		Entry("assign IP to a pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00001"), 0),
		Entry("succeed to run a pod with long yaml for ipv4, ipv6 and dual-stack case", Label("E00007"), 100),
	)

	DescribeTable("assign IP to controller/pod for ipv4, ipv6 and dual-stack case",
		func(controllerType string, replicas int32) {
			// try to create controller
			GinkgoWriter.Printf("try to create controller %v: %v/%v \n", controllerType, testName, namespace)
			switch {
			case controllerType == common.DeploymentNameString:
				dpm = common.GenerateExampleDeploymentYaml(testName, namespace, replicas)
				err = frame.CreateDeployment(dpm)
			case controllerType == common.StatefulSetNameString:
				sts = common.GenerateExampleStatefulSetYaml(testName, namespace, replicas)
				err = frame.CreateStatefulSet(sts)
			case controllerType == common.DaemonSetNameString:
				ds = common.GenerateExampleDaemonSetYaml(testName, namespace)
				err = frame.CreateDaemonSet(ds)
			case controllerType == common.ReplicaSetNameString:
				rs = common.GenerateExampleReplicaSetYaml(testName, namespace, replicas)
				err = frame.CreateReplicaSet(rs)
			default:
				Fail("input variable is not valid")
			}
			Expect(err).NotTo(HaveOccurred(), "failed to create controller %v", controllerType)

			// setting timeout
			ctx1, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			GinkgoWriter.Printf("try to wait controller %v: %v/%v \n", controllerType, testName, namespace)
			switch {
			case controllerType == common.DeploymentNameString:
				dpm, err = frame.WaitDeploymentReady(testName, namespace, ctx1)
				Expect(dpm).NotTo(BeNil())
			case controllerType == common.StatefulSetNameString:
				sts, err = frame.WaitStatefulSetReady(testName, namespace, ctx1)
				Expect(sts).NotTo(BeNil())
			case controllerType == common.DaemonSetNameString:
				ds, err = frame.WaitDaemonSetReady(testName, namespace, ctx1)
				Expect(ds).NotTo(BeNil())
			case controllerType == common.ReplicaSetNameString:
				rs, err = frame.WaitReplicaSetReady(testName, namespace, ctx1)
				Expect(rs).NotTo(BeNil())
			}
			Expect(err).NotTo(HaveOccurred(), "time out to wait controller %v ready", controllerType)

			// try to get controller pod list
			switch {
			case controllerType == common.DeploymentNameString:
				podlist, err = frame.GetDeploymentPodList(dpm)
				Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))
			case controllerType == common.StatefulSetNameString:
				podlist, err = frame.GetStatefulSetPodList(sts)
				Expect(int32(len(podlist.Items))).Should(Equal(sts.Status.ReadyReplicas))
			case controllerType == common.DaemonSetNameString:
				podlist, err = frame.GetDaemonSetPodList(ds)
				Expect(int32(len(podlist.Items))).Should(Equal(ds.Status.NumberReady))
			case controllerType == common.ReplicaSetNameString:
				podlist, err = frame.GetReplicaSetPodList(rs)
				Expect(int32(len(podlist.Items))).Should(Equal(rs.Status.ReadyReplicas))
			}
			Expect(err).NotTo(HaveOccurred(), "failed to list controller pod %v", controllerType)

			// check all pods to created by controller，it`s assign ipv4 and ipv6 addresses success
			err = frame.CheckPodListIpReady(podlist)
			Expect(err).NotTo(HaveOccurred(), "failed to check ipv4 or ipv6")

			// try to delete controller
			GinkgoWriter.Printf("try to delete controller %v: %v/%v \n", controllerType, testName, namespace)
			switch {
			case controllerType == common.DeploymentNameString:
				err = frame.DeleteDeployment(testName, namespace)
			case controllerType == common.StatefulSetNameString:
				err = frame.DeleteStatefulSet(testName, namespace)
			case controllerType == common.DaemonSetNameString:
				err = frame.DeleteDaemonSet(testName, namespace)
			case controllerType == common.ReplicaSetNameString:
				err = frame.DeleteReplicaSet(testName, namespace)
			}
			Expect(err).NotTo(HaveOccurred(), "failed to delete controller %v: %v/%v \n", controllerType, testName, namespace)
		},
		Entry("assign IP to deployment/pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00002"), common.DeploymentNameString, int32(2)),
		Entry("assign IP to statefulSet/pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00003"), common.StatefulSetNameString, int32(2)),
		Entry("assign IP to daemonset/pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00004"), common.DaemonSetNameString, int32(0)),
		Entry("assign IP to replicaset/pod for ipv4, ipv6 and dual-stack case", Label("smoke", "E00006"), common.ReplicaSetNameString, int32(2)),
	)
})
