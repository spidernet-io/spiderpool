// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package performance_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("performance test case", Serial, Label("performance"), func() {
	var perName, nsName string
	var err error
	var dpm *appsv1.Deployment
	var ds *appsv1.StatefulSet
	var podlist *corev1.PodList

	BeforeEach(func() {
		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// init ippool performance test name
		perName = "per" + tools.RandomName()

		// clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	// time cost of create、reboot、delete different number pods through different controller
	DescribeTable("about controller performance test form table,input different numbers of replicas",
		// waiting to expand GC content
		func(controllerType string, replicas int32, overtimeCheck time.Duration) {
			// try to create controller
			GinkgoWriter.Printf("try to create controller %v: %v/%v,replicas is %v \n", controllerType, nsName, perName, replicas)
			switch {
			case controllerType == common.DeploymentNameString:
				podYaml := common.GenerateExampleDeploymentYaml(perName, nsName, replicas)
				err = frame.CreateDeployment(podYaml)
			case controllerType == common.StatefulsetNameString:
				podYaml := common.GenerateExampleStatefulSetYaml(perName, nsName, replicas)
				err = frame.CreateStatefulSet(podYaml)
			default:
				Fail("input variable is not valid")
			}
			Expect(err).NotTo(HaveOccurred(), "failed to create controller %v : %v/%v", controllerType, nsName, perName)

			// setting timeout, as the replicas increase，can change the waiting time。
			// but the same case，last time it worked, this time it didn't，please check performance
			ctx1, cancel1 := context.WithTimeout(context.Background(), overtimeCheck)
			defer cancel1()

			// computing create controller time cost
			startT1 := time.Now()
			switch {
			case controllerType == common.DeploymentNameString:
				dpm, err = frame.WaitDeploymentReady(perName, nsName, ctx1)
				Expect(dpm).NotTo(BeNil())
			case controllerType == common.StatefulsetNameString:
				ds, err = frame.WaitStatefulSetReady(perName, nsName, ctx1)
				Expect(ds).NotTo(BeNil())
			}
			Expect(err).NotTo(HaveOccurred(), "time out to wait %v : %v/%v ready", controllerType, nsName, perName)
			endT1 := time.Since(startT1)

			// get controller pod list for reboot and check ip
			switch {
			case controllerType == common.DeploymentNameString:
				podlist, err = frame.GetDeploymentPodList(dpm)
				Expect(err).NotTo(HaveOccurred(), "failed to list pod")
				Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))
			case controllerType == common.StatefulsetNameString:
				podlist, err = frame.GetStatefulSetPodList(ds)
				Expect(err).NotTo(HaveOccurred(), "failed to list pod")
				Expect(int32(len(podlist.Items))).Should(Equal(ds.Status.ReadyReplicas))
			}

			// check all pods to created by controller，it`s assign ipv4 and ipv6 addresses success
			err = frame.CheckPodListIpReady(podlist)
			Expect(err).NotTo(HaveOccurred(), "failed to checkout ipv4、ipv6")

			// try to reboot controller
			GinkgoWriter.Printf("try to reboot controller %v : %v/%v, reboot replicas is %v \n", controllerType, nsName, perName, replicas)
			err = frame.DeletePodList(podlist)
			Expect(err).NotTo(HaveOccurred(), "failed to reboot controller %v: %v/%v", controllerType, nsName, perName)

			// waiting for controller replicas to be ready
			startT2 := time.Now()
			switch {
			case controllerType == common.DeploymentNameString:
				dpm, err = frame.WaitDeploymentReady(perName, nsName, ctx1)
				Expect(dpm).NotTo(BeNil())
			case controllerType == common.StatefulsetNameString:
				ds, err = frame.WaitStatefulSetReady(perName, nsName, ctx1)
				Expect(ds).NotTo(BeNil())
			}
			endT2 := time.Since(startT2)

			// check all pods to reboot by controller，its assign ipv4 and ipv6 addresses success
			err = frame.CheckPodListIpReady(podlist)
			Expect(err).NotTo(HaveOccurred(), "failed to checkout ipv4、ipv6")

			// try to delete controller
			GinkgoWriter.Printf("try to delete controller %v: %v/%v \n", controllerType, nsName, perName)
			switch {
			case controllerType == common.DeploymentNameString:
				err = frame.DeleteDeployment(perName, nsName)
				Expect(err).NotTo(HaveOccurred(), "failed to delete controller %v: %v/%v", controllerType, nsName, perName)
			case controllerType == common.StatefulsetNameString:
				err = frame.DeleteStatefulSet(perName, nsName)
				Expect(err).NotTo(HaveOccurred(), "failed to delete controller %v: %v/%v", controllerType, nsName, perName)
			}

			ctx3, cancel3 := context.WithTimeout(context.Background(), overtimeCheck)
			defer cancel3()

			// notice: the controller deletion is instantaneous
			// check time cose of all replicas are deleted time
			startT3 := time.Now()
			switch {
			case controllerType == common.DeploymentNameString:
				err = frame.WaitPodDeleteUntilComplete(nsName, dpm.Spec.Selector.MatchLabels, ctx3)
			case controllerType == common.StatefulsetNameString:
				err = frame.WaitPodDeleteUntilComplete(nsName, ds.Spec.Selector.MatchLabels, ctx3)
			}
			Expect(err).NotTo(HaveOccurred(), "time out to wait controller %v replicas delete", controllerType)
			endT3 := time.Since(startT3)

			// output the performance results
			GinkgoWriter.Printf("time cost for create %v: %v/%v of %v replicas = %v \n", controllerType, nsName, perName, replicas, endT1)
			GinkgoWriter.Printf("time cost for reboot %v: %v/%v of %v replicas = %v \n", controllerType, nsName, perName, replicas, endT2)
			GinkgoWriter.Printf("time cost for delete %v: %v/%v of %v replicas = %v \n", controllerType, nsName, perName, replicas, endT3)
			// attaching Data to Reports
			AddReportEntry("Performance Results",
				fmt.Sprintf(`{ "controllerType" : "%s", "replicas": %d, "createTime": %d , "rebuildTime": %d, "deleteTime": %d }`,
					controllerType, replicas, int(endT1.Seconds()), int(endT2.Seconds()), int(endT3.Seconds())))
		},
		// TODO (tao.yang), N controller replicas in Ippool for N IP, Through this template complete gc performance closed-loop test together
		Entry("time cost of create、reboot、delete 60 pods through deployment", Label("P00002"), common.DeploymentNameString, int32(60), time.Second*300),
		Entry("time cost of create、reboot、delete 30 pods through statefulSet", Label("P00003"), common.StatefulsetNameString, int32(30), time.Second*1200),
	)
})
