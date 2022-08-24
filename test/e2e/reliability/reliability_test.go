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
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("test reliability", Label("reliability"), Serial, func() {
	var podName, namespace string
	var wg sync.WaitGroup

	BeforeEach(func() {
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, time.Second*10)
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
			Expect(podList.Items).NotTo(HaveLen(0))
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

			// killed service need recovery, especially spiderpool-controller, or else make other IT failed
			time.Sleep(time.Duration(5 * time.Second))

		},
		Entry("finally succeed to run a pod during the ETCD is restarting",
			Label("R00002"), "etcd", map[string]string{"component": "etcd"}, time.Second*90),
		Entry("finally succeed to run a pod during the API-server is restarting",
			Label("R00003"), "apiserver", map[string]string{"component": "kube-apiserver"}, time.Second*90),
		Entry("finally succeed to run a pod during the coreDns is restarting",
			Label("R00005"), "coredns", map[string]string{"k8s-app": "kube-dns"}, time.Second*90),
		Entry("finally succeed to run a pod during the spiderpool-agent is restarting",
			Label("R00001"), "spiderpool-agent", map[string]string{"app.kubernetes.io/component": "spiderpool-agent"}, time.Second*90),
		Entry("finally succeed to run a pod during the spiderpool-controller is restarting",
			Label("R00004"), "spiderpool-controller", map[string]string{"app.kubernetes.io/component": "spiderpool-controller"}, time.Second*90),
	)

	DescribeTable("check ip assign after reboot node",
		func(replicas int32) {
			daemonSetName := "ds" + tools.RandomName()

			// Genarte Deployment Yaml
			GinkgoWriter.Printf("Create Deployment %v/%v \n", namespace, podName)
			dep := common.GenerateExampleDeploymentYaml(podName, namespace, replicas)

			// In a `kind` cluster, restarting the master node will cause the cluster to become unavailable.
			nodeList, err := frame.GetNodeList()
			Expect(err).ShouldNot(HaveOccurred())
			for _, node := range nodeList.Items {
				if _, ok := node.GetLabels()["node-role.kubernetes.io/control-plane"]; !ok {
					dep.Spec.Template.Spec.NodeSelector = map[string]string{corev1.LabelHostname: node.Name}
					break
				}
			}
			// Create Deployment
			Expect(frame.CreateDeployment(dep)).NotTo(HaveOccurred(), "Failed to create Deployment")

			// Wait for the deployment to be ready and check that the IP assignment is correct
			podList, err1 := frame.WaitDeploymentReadyAndCheckIP(podName, namespace, time.Second*30)
			Expect(err1).ShouldNot(HaveOccurred())

			// Check if the IP exists in the IPPool before restarting the node
			ClusterDefaultIPv4IPPool, ClusterDefaultIPv6IPPool, err2 := common.GetClusterDefaultIppool(frame)
			Expect(err2).ShouldNot(HaveOccurred())
			isRecord1, _, _, err3 := common.CheckPodIpRecordInIppool(frame, ClusterDefaultIPv4IPPool, ClusterDefaultIPv6IPPool, podList)
			Expect(isRecord1).Should(BeTrue())
			Expect(err3).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("Pod IP recorded in IPPool %v,%v \n", ClusterDefaultIPv4IPPool, ClusterDefaultIPv6IPPool)

			// Send a cmd to restart the node and check the cluster until it is ready
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
			defer cancel()
			err4 := common.RestartNodeUntilClusterReady(ctx, frame, podList.Items[0].Spec.NodeName)
			Expect(err4).NotTo(HaveOccurred(), "Execution of cmd fails or node/Pod is not ready, error is: %v \n", err4)

			// After the nodes reboot,create daemonset for checking spiderpool service ready, in case this rebooting test case influence on later test case
			ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute*5)
			defer cancel1()
			dsObj := common.GenerateExampleDaemonSetYaml(daemonSetName, namespace)
			_, err5 := frame.CreateDaemonsetUntilReady(ctx1, dsObj)
			Expect(err5).NotTo(HaveOccurred())

			// After the node is ready then wait for the Deployment to be ready and check that the IP is correctly assigned.
			restartPodList, err6 := frame.WaitDeploymentReadyAndCheckIP(podName, namespace, time.Minute)
			Expect(err6).ShouldNot(HaveOccurred())

			// After restarting the node, check that the IP is still recorded in the ippool.
			isRecord2, _, _, err7 := common.CheckPodIpRecordInIppool(frame, ClusterDefaultIPv4IPPool, ClusterDefaultIPv6IPPool, restartPodList)
			Expect(isRecord2).Should(BeTrue())
			Expect(err7).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("After restarting the node, the IP recorded in the ippool: %v ,%v", ClusterDefaultIPv4IPPool, ClusterDefaultIPv6IPPool)

			// Try to delete Deployment and Daemonset
			Expect(frame.DeleteDeployment(podName, namespace)).NotTo(HaveOccurred())
			Expect(frame.DeleteDaemonSet(daemonSetName, namespace)).NotTo(HaveOccurred())
		},
		PEntry("Successfully recovery a pod whose original node is power-off", Serial, Label("R00006"), int32(2)),
	)
})
