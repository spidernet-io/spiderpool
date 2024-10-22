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
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apitypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/kubectl/pkg/util/podutils"
)

var _ = Describe("test reliability", Label("reliability"), Serial, func() {
	var podName, namespace string
	var wg sync.WaitGroup
	var globalDefaultV4IppoolList, globalDefaultV6IppoolList []string

	BeforeEach(func() {
		namespace = "ns" + tools.RandomName()
		GinkgoWriter.Printf("create namespace %v \n", namespace)
		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		Expect(err).NotTo(HaveOccurred(), "failed to create namespace %v", namespace)
		podName = "pod" + tools.RandomName()

		if frame.Info.IpV4Enabled {
			globalDefaultV4IppoolList = nil
			globalDefaultV4IppoolList = append(globalDefaultV4IppoolList, common.SpiderPoolIPv4PoolDefault)
		}
		if frame.Info.IpV6Enabled {
			globalDefaultV6IppoolList = nil
			globalDefaultV6IppoolList = append(globalDefaultV6IppoolList, common.SpiderPoolIPv6PoolDefault)
		}

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			GinkgoWriter.Printf("delete namespace %v \n", namespace)
			err := frame.DeleteNamespace(namespace)
			Expect(err).NotTo(HaveOccurred(), "failed to delete namespace %v", namespace)
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

	It("Spiderpool Controller active/standby switching is normal", Label("R00007"), func() {

		podList, err := frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController})
		Expect(err).NotTo(HaveOccurred())

		if len(podList.Items) <= 1 {
			Skip("There is only one replicas of spidercontroller, so there is no need to switch between primary and secondary.")
		}

		spiderControllerLeases, err := getLeases(common.SpiderPoolLeasesNamespace, common.SpiderPoolLeases)
		Expect(err).NotTo(HaveOccurred())

		leaseMap := make(map[string]bool)
		for _, v := range podList.Items {
			if *spiderControllerLeases.Spec.HolderIdentity == v.Name {
				GinkgoWriter.Printf("the spiderpool-controller current master is: %v \n", *spiderControllerLeases.Spec.HolderIdentity)
				leaseMap[v.Name] = true
			} else {
				leaseMap[v.Name] = false
			}
		}
		GinkgoWriter.Printf("The master-slave information of spidercontroller is as follows: %v \n", leaseMap)

		for m, n := range leaseMap {
			if n {
				Expect(frame.DeletePod(m, podList.Items[0].Namespace)).NotTo(HaveOccurred())
				ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
				defer cancel()
				err = frame.WaitPodListRunning(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController}, len(podList.Items), ctx)
				Expect(err).NotTo(HaveOccurred())

				Eventually(func() bool {
					spiderControllerLeases, err = getLeases(common.SpiderPoolLeasesNamespace, common.SpiderPoolLeases)
					if err != nil {
						return false
					}

					if spiderControllerLeases.Spec.HolderIdentity == nil || *spiderControllerLeases.Spec.HolderIdentity == "" {
						GinkgoWriter.Println("spiderControllerLeases.Spec.HolderIdentity is a null pointer or empty string")
						return false
					}

					if *spiderControllerLeases.Spec.HolderIdentity == m {
						// After the Pod is restarted, the master should be re-elected.
						return false
					}

					// When there are 3 or more replicas of a spidercontroller,
					// it is impossible to determine which replica is the master.
					// But they must be on the map or elected as new Pods
					if _, ok := leaseMap[*spiderControllerLeases.Spec.HolderIdentity]; !ok {
						GinkgoWriter.Printf("The lease records a value: %v that does not exist in leaseMap \n", *spiderControllerLeases.Spec.HolderIdentity)
						podList, err = frame.GetPodListByLabel(map[string]string{"app.kubernetes.io/component": constant.SpiderpoolController})
						if err != nil {
							return false
						}
						for _, pod := range podList.Items {
							if _, ok := leaseMap[pod.Name]; !ok {
								if *spiderControllerLeases.Spec.HolderIdentity != pod.Name {
									Fail("The leader election failed. Neither the surviving Pods nor the new Pods were selected.")
								}
								return true
							}
						}
					}

					GinkgoWriter.Printf("spiderpool-controller master-slave switchover is successful, the current master is: %v \n", *spiderControllerLeases.Spec.HolderIdentity)
					return true
				}).WithTimeout(time.Minute).WithPolling(time.Second * 3).Should(BeTrue())
			}
		}
	})

	It("The metric should work fine.", Label("K00001"), func() {
		ctx, cancel := context.WithTimeout(context.Background(), common.PodReStartTimeout)
		defer cancel()
		Expect(checkMetrics(ctx, common.SpiderControllerMetricsPort)).NotTo(HaveOccurred())
		GinkgoWriter.Println("spidercontroller metrics access successful.")

		ctx, cancel = context.WithTimeout(context.Background(), common.PodReStartTimeout)
		defer cancel()
		Expect(checkMetrics(ctx, common.SpiderAgentMetricsPort)).NotTo(HaveOccurred())
		GinkgoWriter.Println("spiderAgent metrics access successful.")
	})
})

func getLeases(namespace, leaseName string) (*coordinationv1.Lease, error) {
	v := apitypes.NamespacedName{Name: leaseName, Namespace: namespace}
	existing := &coordinationv1.Lease{}
	e := frame.GetResource(v, existing)
	if e != nil {
		return nil, e
	}
	return existing, nil
}

func checkMetrics(ctx context.Context, metricsPort string) error {
	const metricsRoute = "/metrics"

	nodeList, err := frame.GetNodeList()
	if err != nil {
		return fmt.Errorf("failed to get node information")
	}

	var metricsHealthyCheck string
	if frame.Info.IpV6Enabled && !frame.Info.IpV4Enabled {
		metricsHealthyCheck = fmt.Sprintf("curl -I -m 1 -g [%s]:%s%s --insecure", nodeList.Items[0].Status.Addresses[0].Address, metricsPort, metricsRoute)
	} else {
		metricsHealthyCheck = fmt.Sprintf("curl -I -m 1 %s:%s%s --insecure", nodeList.Items[0].Status.Addresses[0].Address, metricsPort, metricsRoute)
	}

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for metrics Healthy Check to be ready")
		default:
			out, err := frame.DockerExecCommand(ctx, nodeList.Items[0].Name, metricsHealthyCheck)
			if err != nil {
				time.Sleep(common.ForcedWaitingTime)
				frame.Log("failed to check metrics healthy, error: %v \n output log is: %v ", err, string(out))
				continue
			}
			return nil
		}
	}
}
