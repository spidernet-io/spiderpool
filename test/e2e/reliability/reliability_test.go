// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reliability_test

import (
	"context"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test reliability", Label("reliability"), Serial, func() {
	var podName, namespace string
	var wg sync.WaitGroup

	BeforeEach(func() {
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespace(namespace)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
		podName = "pod" + tools.RandomName()

		DeferCleanup(func() {
			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
		})
	})

	DescribeTable("reliability test table",
		func(componentName string, label map[string]string, startupTimeRequired time.Duration) {

			// get component pod list
			GinkgoWriter.Printf("get %v pod list \n", componentName)
			podList, e1 := frame.GetPodListByLabel(label)
			Expect(e1).NotTo(HaveOccurred())
			Expect(podList).NotTo(BeNil())
			expectPodNum := len(podList.Items)
			GinkgoWriter.Printf("the %v pod number is: %v \n", componentName, expectPodNum)

			// delete component pod repeatedly every 2 seconds for 6 seconds
			GinkgoWriter.Printf("delete %v %v pod repeatedly every 2 seconds for 6 seconds \n", expectPodNum, componentName)
			ctx1, cancel1 := context.WithTimeout(context.Background(), time.Second*6)
			defer cancel1()
			e2 := frame.DeletePodListRepeatedly(label, time.Second*2, ctx1)
			Expect(e2).NotTo(HaveOccurred())

			// wait component pod ready
			GinkgoWriter.Printf("wait %v pod list ready \n", componentName)
			ctx2, cancel2 := context.WithTimeout(context.Background(), startupTimeRequired)
			defer cancel2()
			e3 := frame.WaitPodListRunning(label, expectPodNum, ctx2)
			Expect(e3).NotTo(HaveOccurred())
			GinkgoWriter.Printf("%v pods are ready \n", componentName)

			// create pod  when component is unstable
			GinkgoWriter.Printf("create pod %v/%v when %v is unstable \n", namespace, podName, componentName)
			podYaml := common.GenerateExamplePodYaml(podName, namespace)
			e4 := frame.CreatePod(podYaml)
			Expect(e4).NotTo(HaveOccurred())

			// at the same time, use goroutine delete component pod repeatedly every 2 seconds for 6 seconds
			GinkgoWriter.Printf("at the same time delete %v pod repeatedly every 2 seconds for 6 seconds \n", componentName)
			ctx3, cancel3 := context.WithTimeout(context.Background(), time.Second*6)
			defer cancel3()

			wg.Add(1)
			go func() {
				GinkgoRecover()
				e5 := frame.DeletePodListRepeatedly(label, time.Second*2, ctx3)
				Expect(e5).NotTo(HaveOccurred())
				wg.Done()
			}()

			// wait test pod ready
			ctx4, cancel4 := context.WithTimeout(context.Background(), time.Minute*2)
			defer cancel4()
			_, e6 := frame.WaitPodStarted(podName, namespace, ctx4)
			Expect(e6).NotTo(HaveOccurred())

			wg.Wait()

			// last confirm the component pod running normally
			GinkgoWriter.Printf("finally wait %v running normally \n", componentName)
			ctx5, cancel5 := context.WithTimeout(context.Background(), startupTimeRequired)
			defer cancel5()
			e7 := frame.WaitPodListRunning(label, expectPodNum, ctx5)
			Expect(e7).NotTo(HaveOccurred())
			GinkgoWriter.Printf("component %v running normally \n", componentName)
		},
		Entry("finally succeed to run a pod during the ETCD is restarting",
			Label("R00002"), "etcd", map[string]string{"component": "etcd"}, time.Second*90),
		Entry("finally succeed to run a pod during the API-server is restarting",
			Label("R00003"), "apiserver", map[string]string{"component": "kube-apiserver"}, time.Second*90),
		Entry("finally succeed to run a pod during the coreDns is restarting",
			Label("R00005"), "coredns", map[string]string{"k8s-app": "kube-dns"}, time.Second*90),
		// TODO(bingzhesun) spiderpool
	)

	DescribeTable("check ip assign after reboot node",
		func(replicas int32) {
			GinkgoWriter.Printf("create Deployment")
			// create Deployment
			GinkgoWriter.Printf("try to create Deployment %v/%v \n", podName, namespace)
			dep := common.GenerateExampleDeploymentYaml(podName, namespace, replicas)
			err := frame.CreateDeployment(dep)
			Expect(err).NotTo(HaveOccurred(), "failed to create Deployment")

			// wait deployment ready and check ip assign ok
			podlist, errip := frame.WaitDeploymentReadyAndCheckIP(podName, namespace)
			Expect(errip).ShouldNot(HaveOccurred())

			// before reboot node check ip in ipppool
			defaultv4, defaultv6, err := common.GetClusterDefaultIppool(frame)
			Expect(err).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("default ip4 ippool name is %v\n default ip6 ippool name is %v\n,", defaultv4, defaultv6)
			inippool, errpool := common.CheckPodIpRecordInIppool(frame, defaultv4, defaultv6, podlist)
			Eventually(inippool).Should(BeTrue())
			Expect(errpool).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("check pod inippool：%v\n", inippool)

			// get nodename list
			nodeMap := make(map[string]bool)
			for _, item := range podlist.Items {
				GinkgoWriter.Printf("item.Status.NodeName:%v\n", item.Spec.NodeName)
				nodeMap[item.Spec.NodeName] = true
			}
			GinkgoWriter.Printf("get nodeMap is：%v\n", nodeMap)

			// send cmd to reboot node
			common.ExecCommandRebootNode(nodeMap)

			// check node ready
			readyok, err := frame.IsClusterNodeReady()
			Expect(err).ShouldNot(HaveOccurred())
			Eventually(readyok).Should(BeTrue())

			// check pod ip assign ok
			_, errip2 := frame.WaitDeploymentReadyAndCheckIP(podName, namespace)
			Expect(errip2).ShouldNot(HaveOccurred())

			// after reboot node check ip in ipppool
			defaultv4a, defaultv6a, err := common.GetClusterDefaultIppool(frame)
			Expect(err).ShouldNot(HaveOccurred())
			inippoola, errpoola := common.CheckPodIpRecordInIppool(frame, defaultv4a, defaultv6a, podlist)
			Eventually(inippoola).Should(BeTrue())
			Expect(errpoola).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("check pod inippool：%v\n", inippool)

			//delete Deployment
			errdel := frame.DeleteDeployment(podName, namespace)
			Expect(errdel).NotTo(HaveOccurred(), "failed to delete Deployment %v: %v/%v \n", podName, namespace)

		},
		//Entry("pod Replicas is 1", Label("R00006"), int32(1)),
		Entry("pod Replicas is 2", Label("R00006"), int32(2)),
	)
})
