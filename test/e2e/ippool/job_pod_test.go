// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippool_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	"k8s.io/utils/pointer"
)

var _ = Describe("test ip with Job case", Label("Job"), func() {

	var jdName, nsName string

	BeforeEach(func() {

		// init namespace name and create
		nsName = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", nsName)
		err := frame.CreateNamespace(nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", nsName)

		// init Job name
		jdName = "jd" + tools.RandomName()

		//clean test env
		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", nsName)
			err = frame.DeleteNamespace(nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", nsName)
		})
	})

	It("one Job 2 pods allocate/release ipv4 and ipv6 addresses", Label("smoke", "E00005"), func() {

		// create Job
		GinkgoWriter.Printf("try to create Job %v/%v \n", jdName, nsName)

		behavior := common.JobTypeRunningForever
		jd := common.GenerateExampleJobYaml(behavior, jdName, nsName, pointer.Int32Ptr(2))
		Expect(jd).NotTo(BeNil())
		label := jd.Spec.Template.Labels
		parallelism := jd.Spec.Parallelism

		GinkgoWriter.Printf("job yaml:\n %v \n", jd)

		e1 := frame.CreateJob(jd)
		Expect(e1).NotTo(HaveOccurred(), "failed to create job \n")

		// wait job pod list running
		GinkgoWriter.Printf("wait job pod list running \n")
		ctx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel1()
		e2 := frame.WaitPodListRunning(label, int(*parallelism), ctx1)
		Expect(e2).NotTo(HaveOccurred())

		// get job pod list
		GinkgoWriter.Printf("get job pod list \n")
		podlist, e3 := frame.GetJobPodList(jd)
		Expect(e3).NotTo(HaveOccurred())
		Expect(podlist).NotTo(BeNil())

		for i := 0; i < len(podlist.Items); i++ {
			GinkgoWriter.Printf("pod %v/%v ips: %+v \n", nsName, podlist.Items[i].Name, podlist.Items[i].Status.PodIPs)
			err := frame.CheckPodListIpReady(podlist)
			Expect(err).NotTo(HaveOccurred(), "failed to checkout ipv4、ipv6")
			if frame.Info.IpV4Enabled == true {
				Expect(tools.CheckPodIpv4IPReady(&podlist.Items[i])).To(BeTrue(), "pod %v failed to get ipv4 ip", podlist.Items[i].Name)
				By("succeeded to check pod ipv4 ip ")
			}
			if frame.Info.IpV6Enabled == true {
				Expect(tools.CheckPodIpv6IPReady(&podlist.Items[i])).To(BeTrue(), "pod %v failed to get ipv6 ip", podlist.Items[i].Name)
				By("succeeded to check pod ipv6 ip \n")
			}
		}

		// delete job
		GinkgoWriter.Printf("delete job: %v \n", jdName)
		err := frame.DeleteJob(jdName, nsName)
		Expect(err).NotTo(HaveOccurred(), "failed to delete job: %v \n", jdName)
	})

	DescribeTable("check ip release after job finished",

		func(behavior common.JobBehave) {
			// create Job
			GinkgoWriter.Printf("try to create Job %v/%v \n", nsName, jdName)
			jd := common.GenerateExampleJobYaml(behavior, jdName, nsName, pointer.Int32Ptr(2))
			Expect(jd).NotTo(BeNil())
			GinkgoWriter.Printf("job behavior:\n %v \n", behavior)
			e1 := frame.CreateJob(jd)
			Expect(e1).NotTo(HaveOccurred(), "failed to create job \n")

			// wait job finished
			GinkgoWriter.Printf("wait job finished \n")
			ctx1, cancel1 := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel1()
			jb, ok1, e3 := frame.WaitJobFinished(jdName, nsName, ctx1)
			GinkgoWriter.Printf("job finished status:%v \n", ok1)

			switch behavior {
			case common.JobTypeFail:
				Expect(ok1).To(BeFalse())
			case common.JobTypeFinish:
				Expect(ok1).To(BeTrue())
			default:
				Fail("input error")
			}

			Expect(e3).NotTo(HaveOccurred())
			Expect(jb).NotTo(BeNil())
			GinkgoWriter.Printf("job %v is finished \n job conditions:\n %v \n", jb, jb.Status.Conditions)

			// TODO(weiyang) check ip release

			//delete job
			GinkgoWriter.Printf("delete job: %v \n", jdName)
			err := frame.DeleteJob(jdName, nsName)
			Expect(err).NotTo(HaveOccurred(), "failed to delete job: %v \n", jdName)

		},

		Entry("check ip release when job is failed", Label("E00005"), common.JobTypeFail),
		Entry("check ip release when job is succeeded", Label("E00005"), common.JobTypeFinish),

		// TODO(yangwei) check to release

	)

})
