// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippool_test

import (
	"context"
	"fmt"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("about ippool performance test case", Serial, Label("performance"), func() {
	var perName, nsName string
	var err error

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
	Context("through the deployment controller, check time cost to CRUD different numbers of pods", func() {
		DescribeTable("about deployment performance test form table,input different numbers of replicas",
			func(createReplicas, scaleReplicas int32, overtimeCheck time.Duration) {
				// create deployment
				GinkgoWriter.Printf("try to create deployment: %v/%v,replicas is %v \n", nsName, perName, createReplicas)
				dpm := common.GenerateExampleDeploymentYaml(perName, nsName, createReplicas)
				err = frame.CreateDeployment(dpm)
				Expect(err).NotTo(HaveOccurred(), "failed to create deployment")

				// setting timeout, as the replicas increase，can change the waiting time。
				// but the same case，last time it worked, this time it didn't，please check performance
				ctx1, cancel1 := context.WithTimeout(context.Background(), overtimeCheck)
				defer cancel1()

				// computing create deployment time cost
				startT1 := time.Now()
				dpm, err := frame.WaitDeploymentReady(perName, nsName, ctx1)
				Expect(err).NotTo(HaveOccurred(), "time out to wait deployment ready")
				Expect(dpm).NotTo(BeNil())
				endT1 := time.Since(startT1)
				//GinkgoWriter.Printf("time cost for create deployment of %v replicas= %v \n", creReplicas, endT1)

				// scale deployment
				GinkgoWriter.Printf("try to scale deployment: %v/%v,replicas is %v \n", nsName, perName, scaleReplicas)
				dpm, err = frame.ScaleDeployment(dpm, scaleReplicas)
				Expect(err).NotTo(HaveOccurred(), "failed to scale deployment replicas")
				Expect(dpm).NotTo(BeNil())

				ctx2, cancel2 := context.WithTimeout(context.Background(), overtimeCheck)
				defer cancel2()

				// time cost for scale deployment replicas
				startT2 := time.Now()
				dpm, err = frame.WaitDeploymentReady(perName, nsName, ctx2)
				Expect(err).NotTo(HaveOccurred(), "time out to wait deployment replicas ready")
				Expect(dpm).NotTo(BeNil())
				endT2 := time.Since(startT2)
				// GinkgoWriter.Printf("time cost for scale deployment of %v replicas = %v \n", scaleReplicas, endT2)

				// get deployment pod list
				podlist, err := frame.GetDeploymentPodList(dpm)
				Expect(err).NotTo(HaveOccurred(), "failed to list pod")
				Expect(int32(len(podlist.Items))).Should(Equal(dpm.Status.ReadyReplicas))

				// check all pods created by deployment，its assign ipv4 and ipv6 addresses success
				common.CheckPodListIpReady(frame, podlist)

				// delete deployment
				GinkgoWriter.Printf("try to delete deployment: %v/%v \n", nsName, perName)
				err = frame.DeleteDeployment(perName, nsName)
				Expect(err).NotTo(HaveOccurred(), "failed to delete deployment: %v", perName)

				ctx3, cancel3 := context.WithTimeout(context.Background(), overtimeCheck)
				defer cancel3()

				// notice: Deployment deletion is instantaneous
				// check time cose of all replicas are deleted time
				startT3 := time.Now()
				err = frame.WaitDeleteUntilComplete(nsName, dpm.Spec.Selector.MatchLabels, ctx3)
				Expect(err).NotTo(HaveOccurred(), "time out to wait deployment replicas delete")
				endT3 := time.Since(startT3)

				// output the performance results
				GinkgoWriter.Printf("time cost for create deployment %v/%v of %v replicas = %v \n", nsName, perName, createReplicas, endT1)
				GinkgoWriter.Printf("time cost for scale deployment %v/%v of %v replicas = %v \n", nsName, perName, scaleReplicas, endT2)
				GinkgoWriter.Printf("time cost for delete deployment %v/%v of %v replicas = %v \n", nsName, perName, scaleReplicas, endT3)

				// Print console
				AddReportEntry("output the performance results of operate title",
					consloeStruct{label1: "operateType", label2: "create", label3: "scale", label4: "delete"})
				AddReportEntry("output the performance results of replicas",
					consloeStruct{label1: "replicas", label2: strconv.Itoa(int(createReplicas)),
						label3: strconv.Itoa(int(scaleReplicas)), label4: strconv.Itoa(int(scaleReplicas))})
				AddReportEntry("output the performance results time cost",
					consloeStruct{label1: "timeCost", label2: endT1.String(), label3: endT2.String(), label4: endT3.String()})
			},
			Entry("time cost of CRUD in 100 pods through deployment", Label("P00002", "performance"), int32(30), int32(60), time.Second*300),
			// TODO (tao.yang), the kind cluster could not create more pods
		)
	})

	Context("through the statefulSet controller, check time cost to CRUD different numbers of pods", func() {
		DescribeTable("about statefulSet performance test form table,input different numbers of replicas",
			func(createReplicas, scaleReplicas int32, overtimeCheck time.Duration) {
				// create statefulSet
				GinkgoWriter.Printf("try to create statefulSet: %v/%v,replicas is %v \n", nsName, perName, createReplicas)
				ds := common.GenerateExampleStatefulSetYaml(perName, nsName, createReplicas)
				err = frame.CreateStatefulSet(ds)
				Expect(err).NotTo(HaveOccurred(), "failed to create statefulSet")

				// setting timeout, as the replicas increase，can change the waiting time。
				// but the same case，last time it worked, this time it didn't，please check performance
				ctx1, cancel1 := context.WithTimeout(context.Background(), overtimeCheck)
				defer cancel1()

				// computing create statefulSet time cost
				startT1 := time.Now()
				ds, err := frame.WaitStatefulSetReady(perName, nsName, ctx1)
				Expect(err).NotTo(HaveOccurred(), "time out to wait statefulSet ready")
				Expect(ds).NotTo(BeNil())
				endT1 := time.Since(startT1)

				// scale statefulSet
				GinkgoWriter.Printf("try to scale statefulSet: %v/%v,replicas is %v \n", nsName, perName, scaleReplicas)
				ds, err = frame.ScaleStatefulSet(ds, scaleReplicas)
				Expect(err).NotTo(HaveOccurred(), "failed to scale statefulSet replicas")
				Expect(ds).NotTo(BeNil())

				ctx2, cancel2 := context.WithTimeout(context.Background(), overtimeCheck)
				defer cancel2()

				// time cost for scale statefulSet replicas
				startT2 := time.Now()
				ds, err = frame.WaitStatefulSetReady(perName, nsName, ctx2)
				Expect(err).NotTo(HaveOccurred(), "time out to wait statefulSet replicas ready")
				Expect(ds).NotTo(BeNil())
				endT2 := time.Since(startT2)

				// get statefulSet pod list
				podlist, err := frame.GetStatefulSetPodList(ds)
				Expect(err).NotTo(HaveOccurred(), "failed to list pod")
				Expect(int32(len(podlist.Items))).Should(Equal(ds.Status.ReadyReplicas))

				// check all pods created by statefulSet，its assign ipv4 and ipv6 addresses success
				common.CheckPodListIpReady(frame, podlist)

				// delete statefulSet
				GinkgoWriter.Printf("try to delete statefulSet: %v/%v \n", nsName, perName)
				err = frame.DeleteStatefulSet(perName, nsName)
				Expect(err).NotTo(HaveOccurred(), "failed to delete statefulSet: %v", perName)

				ctx3, cancel3 := context.WithTimeout(context.Background(), overtimeCheck)
				defer cancel3()

				// notice: statefulSet deletion is instantaneous
				// check time cose of all replicas are deleted time
				startT3 := time.Now()
				err = frame.WaitDeleteUntilComplete(nsName, ds.Spec.Selector.MatchLabels, ctx3)
				Expect(err).NotTo(HaveOccurred(), "time out to wait statefulSet replicas delete")
				endT3 := time.Since(startT3)

				// output the performance results
				GinkgoWriter.Printf("time cost for create statefulSet %v/%v of %v replicas = %v \n", nsName, perName, createReplicas, endT1)
				GinkgoWriter.Printf("time cost for scale statefulSet %v/%v of %v replicas = %v \n", nsName, perName, scaleReplicas, endT2)
				GinkgoWriter.Printf("time cost for delete statefulSet %v/%v of %v replicas = %v \n", nsName, perName, scaleReplicas, endT3)

				// Print console
				AddReportEntry("output the statefulSet performance results of operate title",
					consloeStruct{label1: "operateType", label2: "create", label3: "scale", label4: "delete"})
				AddReportEntry("output the statefulSet performance results of replicas",
					consloeStruct{label1: "replicas", label2: strconv.Itoa(int(createReplicas)),
						label3: strconv.Itoa(int(scaleReplicas)), label4: strconv.Itoa(int(scaleReplicas))})
				AddReportEntry("output the statefulSet performance results time cost",
					consloeStruct{label1: "timeCost", label2: endT1.String(), label3: endT2.String(), label4: endT3.String()})
			},
			Entry("time cost of CRUD in 100 pods through statefulSet", Label("P00003", "performance"), int32(10), int32(20), time.Second*1200),
			// TODO (tao.yang), the kind cluster could not create more pods
		)
	})
})

type consloeStruct struct {
	label1 string
	label2 string
	label3 string
	label4 string
}

func (cs consloeStruct) ColorableString() string {
	return fmt.Sprintf("{{red}}%v | {{red}}%v | {{red}}%v | {{red}}%v | {{/}}\n", cs.label1, cs.label2, cs.label3, cs.label4)
}
