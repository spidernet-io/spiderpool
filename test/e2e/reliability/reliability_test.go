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
			podList, e := frame.GetPodListByLabel(label)
			Expect(e).NotTo(HaveOccurred())
			Expect(podList).NotTo(BeNil())
			expectPodNum := len(podList.Items)
			GinkgoWriter.Printf("the %v pod number is: %v \n", componentName, expectPodNum)

			// delete component pod
			GinkgoWriter.Printf("restart %v %v pod  \n", expectPodNum, componentName)
			podList, e = frame.DeletePodListUntilReady(podList, startupTimeRequired)
			Expect(e).NotTo(HaveOccurred())
			Expect(podList).NotTo(BeNil())

			// create pod when component is unstable
			GinkgoWriter.Printf("create pod %v/%v when %v is unstable \n", namespace, podName, componentName)
			podYaml := common.GenerateExamplePodYaml(podName, namespace)
			e = frame.CreatePod(podYaml)
			Expect(e).NotTo(HaveOccurred())

			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				// delete component pod
				startT1 := time.Now()
				GinkgoWriter.Printf("restart %v %v pod  \n", expectPodNum, componentName)
				podList, e1 := frame.DeletePodListUntilReady(podList, startupTimeRequired)
				Expect(e1).NotTo(HaveOccurred())
				Expect(podList).NotTo(BeNil())
				endT1 := time.Since(startT1)
				GinkgoWriter.Printf("component restart until running time cost is:%v\n", endT1)
				wg.Done()
			}()

			// wait test pod ready
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
			defer cancel()
			pod, e := frame.WaitPodStarted(podName, namespace, ctx)
			Expect(e).NotTo(HaveOccurred())
			Expect(pod.Status.PodIPs).NotTo(BeEmpty(), "pod failed to assign ip")
			GinkgoWriter.Printf("pod: %v/%v, ips: %+v \n", namespace, podName, pod.Status.PodIPs)

			// check test pod assign ip ok
			if frame.Info.IpV4Enabled {
				podIPv4, ok := tools.CheckPodIpv4IPReady(pod)
				Expect(ok).NotTo(BeFalse(), "failed to get ipv4 ip")
				Expect(podIPv4).NotTo(BeEmpty(), "podIPv4 is a empty string")
				GinkgoWriter.Println("succeeded to check pod ipv4 ip")
			}
			if frame.Info.IpV6Enabled {
				podIPv6, ok := tools.CheckPodIpv6IPReady(pod)
				Expect(ok).NotTo(BeFalse(), "failed to get ipv6 ip")
				Expect(podIPv6).NotTo(BeEmpty(), "podIPv6 is a empty string")
				GinkgoWriter.Println("succeeded to check pod ipv6 ip")
			}

			wg.Wait()

			// try to delete pod
			GinkgoWriter.Printf("delete pod %v/%v \n", namespace, podName)
			e = frame.DeletePod(podName, namespace)
			Expect(e).NotTo(HaveOccurred(), "failed to delete pod %v/%v \n", namespace, podName)

			// killed service need recovery, espeically spiderpool-controller, or else make other IT failed
			time.Sleep(time.Duration(5 * time.Second))
		},
		Entry("finally succeed to run a pod during the ETCD is restarting",
			Label("R00002"), "etcd", map[string]string{"component": "etcd"}, time.Second*90),
		Entry("finally succeed to run a pod during the API-server is restarting",
			Label("R00003"), "apiserver", map[string]string{"component": "kube-apiserver"}, time.Second*90),
		Entry("finally succeed to run a pod during the coreDns is restarting",
			Label("R00005"), "coredns", map[string]string{"k8s-app": "kube-dns"}, time.Second*90),
		Entry("finally succeed to run a pod during the spiderpool-agent is restarting",
			Label("R00001"), "spiderpool-agent", map[string]string{"app.kubernetes.io/component": "spiderpoolagent"}, time.Second*90),
		Entry("finally succeed to run a pod during the spiderpool-controller is restarting",
			Label("R00004"), "spiderpool-controller", map[string]string{"app.kubernetes.io/component": "spiderpoolcontroller"}, time.Second*90),
	)

	DescribeTable("check ip assign after reboot node",
		func(replicas int32) {
			// create Deployment
			GinkgoWriter.Printf("create Deployment %v/%v \n", namespace, podName)
			dep := common.GenerateExampleDeploymentYaml(podName, namespace, replicas)
			err := frame.CreateDeployment(dep)
			Expect(err).NotTo(HaveOccurred(), "failed to create Deployment")

			// wait deployment ready and check ip assign ok
			podlist, errip := frame.WaitDeploymentReadyAndCheckIP(podName, namespace, time.Second*30)
			Expect(errip).ShouldNot(HaveOccurred())

			// before reboot node check ip exists in ipppool
			defaultv4, defaultv6, err := common.GetClusterDefaultIppool(frame)
			Expect(err).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("default ip4 ippool name is %v\n default ip6 ippool name is %v\n,", defaultv4, defaultv6)
			allipRecord, _, _, errpool := common.CheckPodIpRecordInIppool(frame, defaultv4, defaultv6, podlist)
			Expect(allipRecord).Should(BeTrue())
			Expect(errpool).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("check pod ip in the ippool：%v\n", allipRecord)

			// get nodename list
			nodeMap := make(map[string]bool)
			for _, item := range podlist.Items {
				GinkgoWriter.Printf("item.Status.NodeName:%v\n", item.Spec.NodeName)
				nodeMap[item.Spec.NodeName] = true
			}
			GinkgoWriter.Printf("get nodeMap is：%v\n", nodeMap)

			// send cmd to reboot node and check cluster until ready
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
			defer cancel()
			readyok, err := common.RestartNodeUntilClusterReady(frame, nodeMap, time.Minute, ctx)
			Expect(readyok).Should(BeTrue(), "timeout to wait cluster ready\n")
			Expect(err).ShouldNot(HaveOccurred())

			// after reboot node to wait deployment ready and ip assign ok
			podlistready, errip2 := frame.WaitDeploymentReadyAndCheckIP(podName, namespace, time.Second*30)
			Expect(errip2).ShouldNot(HaveOccurred())

			// after reboot node to check ip exists in ipppool
			allipRecord2, _, _, errpool := common.CheckPodIpRecordInIppool(frame, defaultv4, defaultv6, podlistready)
			Expect(allipRecord2).Should(BeTrue())
			Expect(errpool).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("check pod ip in the ippool：%v\n", allipRecord2)

			// delete Deployment
			errdel := frame.DeleteDeployment(podName, namespace)
			Expect(errdel).NotTo(HaveOccurred(), "failed to delete Deployment %v/%v \n", podName, namespace)

		},
		Entry("pod Replicas is 2", Serial, Label("R00006"), int32(2)),
	)
})
