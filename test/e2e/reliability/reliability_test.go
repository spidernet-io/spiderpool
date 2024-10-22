// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package reliability_test

import (
	"context"
	"fmt"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpool "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/kubectl/pkg/util/podutils"
)

var _ = Describe("test reliability", Label("reliability"), Serial, func() {
	var podName, namespace string
	var wg sync.WaitGroup
	var v4SubnetName, v6SubnetName, globalV4PoolName, globalV6PoolName string
	var globalV4pool, globalV6pool *spiderpool.SpiderIPPool
	var v4SubnetObject, v6SubnetObject *spiderpool.SpiderSubnet
	var globalDefaultV4IppoolList, globalDefaultV6IppoolList []string

	BeforeEach(func() {
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
		podName = "pod" + tools.RandomName()

		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()
		// Adapt to the default subnet, create a new pool as a public pool
		if frame.Info.IpV4Enabled {
			globalV4PoolName, globalV4pool = common.GenerateExampleIpv4poolObject(10)
			if frame.Info.SpiderSubnetEnabled {
				GinkgoWriter.Printf("Create v4 subnet %v and v4 pool %v \n", v4SubnetName, globalV4PoolName)
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, 100)
				Expect(v4SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
				err := common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, globalV4pool, 3)
				Expect(err).NotTo(HaveOccurred())
			} else {
				err := common.CreateIppool(frame, globalV4pool)
				Expect(err).NotTo(HaveOccurred())
			}
			globalDefaultV4IppoolList = append(globalDefaultV4IppoolList, globalV4PoolName)
		}
		if frame.Info.IpV6Enabled {
			globalV6PoolName, globalV6pool = common.GenerateExampleIpv6poolObject(10)
			if frame.Info.SpiderSubnetEnabled {
				GinkgoWriter.Printf("Create v6 subnet %v and v6 pool %v \n", v6SubnetName, globalV6PoolName)
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, 100)
				Expect(v6SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
				err := common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, globalV6pool, 3)
				Expect(err).NotTo(HaveOccurred())
			} else {
				err := common.CreateIppool(frame, globalV6pool)
				Expect(err).NotTo(HaveOccurred())
			}
			globalDefaultV6IppoolList = append(globalDefaultV6IppoolList, globalV6PoolName)
		}

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)

			if frame.Info.IpV6Enabled {
				Expect(common.DeleteIPPoolByName(frame, globalV6PoolName)).NotTo(HaveOccurred())
				if frame.Info.SpiderSubnetEnabled {
					Expect(common.DeleteSubnetByName(frame, v6SubnetName)).NotTo(HaveOccurred())
				}
				globalDefaultV6IppoolList = []string{}
			}
			if frame.Info.IpV4Enabled {
				Expect(common.DeleteIPPoolByName(frame, globalV4PoolName)).NotTo(HaveOccurred())
				if frame.Info.SpiderSubnetEnabled {
					Expect(common.DeleteSubnetByName(frame, v4SubnetName)).NotTo(HaveOccurred())
				}
				globalDefaultV4IppoolList = []string{}
			}
		})
	})

	DescribeTable("reliability test table",
		func(componentName string, label map[string]string, startupTimeRequired time.Duration) {
			// get component pod list
			componentPodList, err := frame.GetPodListByLabel(label)
			Expect(err).NotTo(HaveOccurred(), "failed to get %v pod list", componentName)
			expectPodNum := len(componentPodList.Items)
			GinkgoWriter.Printf("succeeded to get %v pod list \n", componentName)

			// Define a set of daemonSets with Pods on each node to verify that the components on each node can provide services for the Pods.
			dsName := "ds" + tools.RandomName()
			dsYaml := common.GenerateExampleDaemonSetYaml(dsName, namespace)
			podIppoolAnnoStr := common.GeneratePodIPPoolAnnotations(frame, common.NIC1, globalDefaultV4IppoolList, globalDefaultV6IppoolList)
			dsYaml.Spec.Template.Annotations = map[string]string{constant.AnnoPodIPPool: podIppoolAnnoStr}

			// Concurrently delete components and create a new pod
			wg.Add(2)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				GinkgoWriter.Printf("now time: %s, restart %v %v pod \n", time.Now().Format(time.RFC3339Nano), expectPodNum, componentName)
				err := frame.DeletePodList(componentPodList)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					componentPodList, err := frame.GetPodListByLabel(label)
					if err != nil {
						return fmt.Errorf("failed to get component %v pod list", componentName)
					}
					if len(componentPodList.Items) != expectPodNum {
						return fmt.Errorf("the number of component %s pod is not equal to expectPodNum %d", componentName, expectPodNum)
					}
					for _, pod := range componentPodList.Items {
						if !podutils.IsPodReady(&pod) {
							return fmt.Errorf("the pod %v is not ready", pod.Name)
						}
					}

					// Check webhook service ready after restarting the spiderpool-controller, Avoid affecting the creation of IPPool
					if componentName == constant.SpiderpoolController {
						ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
						defer cancel()
						Expect(common.WaitWebhookReady(ctx, frame, common.WebhookPort)).NotTo(HaveOccurred())
					}
					return nil
				}).WithTimeout(common.PodReStartTimeout).WithPolling(time.Second * 3).Should(BeNil())
			}()

			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				GinkgoWriter.Printf("create daemonSet %v/%v when %v is unstable \n", namespace, dsName, componentName)
				err := frame.CreateDaemonSet(dsYaml)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() error {
					podList, err := frame.GetPodListByLabel(dsYaml.Spec.Template.Labels)
					if err != nil {
						return err
					}
					if len(podList.Items) != len(frame.Info.KindNodeList) {
						return fmt.Errorf("the number of pod is not equal to expectPodNum %v", len(frame.Info.KindNodeList))
					}
					for _, pod := range podList.Items {
						if !podutils.IsPodReady(&pod) {
							return fmt.Errorf("the pod %v is not ready", pod.Name)
						}
					}

					// Check the Pod's IP recorded IPPool
					ok, _, _, err := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IppoolList, globalDefaultV6IppoolList, podList)
					if err != nil && !ok {
						return err
					}

					if err := frame.DeleteDaemonSet(dsName, namespace); err != nil {
						return err
					}

					if err := common.WaitIPReclaimedFinish(frame, globalDefaultV4IppoolList, globalDefaultV6IppoolList, podList, common.IPReclaimTimeout); err != nil {
						return err
					}
					return nil
				}).WithTimeout(common.PodStartTimeout).WithPolling(time.Second * 5).Should(BeNil())
			}()
			wg.Wait()
		},
		Entry("Successfully run a pod during the ETCD is restarting",
			Label("R00002"), "etcd", map[string]string{"component": "etcd"}, common.PodStartTimeout),
		Entry("Successfully run a pod during the API-server is restarting",
			Label("R00003"), "apiserver", map[string]string{"component": "kube-apiserver"}, common.PodStartTimeout),
		Entry("Successfully run a pod during the coreDns is restarting",
			Label("R00005"), "coredns", map[string]string{"k8s-app": "kube-dns"}, common.PodStartTimeout),
		Entry("Successfully run a pod during the Spiderpool agent is restarting",
			Label("R00004", "G00008"), constant.SpiderpoolAgent, map[string]string{"app.kubernetes.io/component": constant.SpiderpoolAgent}, common.PodStartTimeout),
		Entry("Successfully run a pod during the Spiderpool controller is restarting",
			Label("R00001", "G00008"), constant.SpiderpoolController, map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController}, common.PodStartTimeout),
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
			// Create the deployment and wait for it to be ready, check that the IP assignment is correct
			Expect(frame.CreateDeployment(dep)).NotTo(HaveOccurred(), "Failed to create Deployment")
			podList, err1 := frame.WaitDeploymentReadyAndCheckIP(podName, namespace, common.PodStartTimeout)
			Expect(err1).ShouldNot(HaveOccurred())

			// Check if the IP exists in the IPPool before restarting the node
			isRecord1, _, _, err2 := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IppoolList, globalDefaultV6IppoolList, podList)
			Expect(isRecord1).Should(BeTrue())
			Expect(err2).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("Pod IP recorded in IPPool %v,%v \n", globalDefaultV4IppoolList, globalDefaultV6IppoolList)

			// Send a cmd to restart the node and check the cluster until it is ready
			ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
			defer cancel()
			err3 := common.RestartNodeUntilClusterReady(ctx, frame, podList.Items[0].Spec.NodeName)
			Expect(err3).NotTo(HaveOccurred(), "Execution of cmd fails or node/Pod is not ready, error is: %v \n", err3)

			// After the nodes reboot,create daemonset for checking spiderpool service ready, in case this rebooting test case influence on later test case
			ctx1, cancel1 := context.WithTimeout(context.Background(), time.Minute*5)
			defer cancel1()
			dsObj := common.GenerateExampleDaemonSetYaml(daemonSetName, namespace)
			_, err4 := frame.CreateDaemonsetUntilReady(ctx1, dsObj)
			Expect(err4).NotTo(HaveOccurred())

			// After the node is ready then wait for the Deployment to be ready and check that the IP is correctly assigned.
			restartPodList, err5 := frame.WaitDeploymentReadyAndCheckIP(podName, namespace, common.PodStartTimeout)
			Expect(err5).ShouldNot(HaveOccurred())

			// After restarting the node, check that the IP is still recorded in the ippool.
			isRecord2, _, _, err6 := common.CheckPodIpRecordInIppool(frame, globalDefaultV4IppoolList, globalDefaultV6IppoolList, restartPodList)
			Expect(isRecord2).Should(BeTrue())
			Expect(err6).ShouldNot(HaveOccurred())
			GinkgoWriter.Printf("After restarting the node, the IP recorded in the ippool: %v ,%v", globalDefaultV4IppoolList, globalDefaultV6IppoolList)

			// Try to delete Deployment and Daemonset
			Expect(frame.DeleteDeployment(podName, namespace)).NotTo(HaveOccurred())
			Expect(frame.DeleteDaemonSet(daemonSetName, namespace)).NotTo(HaveOccurred())
		},
		PEntry("Successfully recovery a pod whose original node is power-off", Serial, Label("R00006"), int32(2)),
	)
})
