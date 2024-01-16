// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ifacer_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/spidernet-io/e2eframework/tools"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("test ifacer", Label("ifacer"), func() {
	var namespace, dsName, spiderMultusNadName, mainInterface string
	var vlanInterface int
	var spiderMultusConfig *spiderpoolv2beta1.SpiderMultusConfig

	BeforeEach(func() {
		dsName = "ds-" + common.GenerateString(10, true)
		namespace = "ns" + tools.RandomName()
		spiderMultusNadName = "test-multus-" + common.GenerateString(10, true)
		mainInterface = common.NIC1

		vlanInterface = 50
		GinkgoWriter.Println("Generate vlan ID of sub-interface:", vlanInterface)

		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		GinkgoWriter.Printf("create namespace %v. \n", namespace)
		Expect(err).NotTo(HaveOccurred())

		spiderMultusConfig = &spiderpoolv2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spiderMultusNadName,
				Namespace: namespace,
			},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: pointer.String(constant.MacvlanCNI),
				MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
					VlanID: pointer.Int32(int32(vlanInterface)),
					SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
						IPv4IPPool: []string{common.SpiderPoolIPv4PoolDefault},
						IPv6IPPool: []string{common.SpiderPoolIPv6PoolDefault},
					},
				},
				CoordinatorConfig: &spiderpoolv2beta1.CoordinatorSpec{},
			},
		}
		GinkgoWriter.Printf("Generate spiderMultusConfig %v \n", spiderMultusConfig)

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}
			GinkgoWriter.Printf("delete namespace %v. \n", namespace)
			Expect(frame.DeleteNamespace(namespace)).NotTo(HaveOccurred())

			// Delete the subinterface used by the test.
			ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
			defer cancel()
			delVlanInterfaceString := fmt.Sprintf("ip link del %s.%v ", mainInterface, vlanInterface)
			Eventually(func() bool {
				for _, node := range frame.Info.KindNodeList {
					_, err := frame.DockerExecCommand(ctx, node, delVlanInterfaceString)
					Expect(err).NotTo(HaveOccurred(), "Failed to execute the delete sub-interface command on the node %s %v", node, err)
				}
				return true
			}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())
		})
	})

	It("About ifacer's e2e use cases", Label("N00001", "N00002", "N00003", "N00006"), func() {
		Expect(frame.CreateSpiderMultusInstance(spiderMultusConfig)).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Create spidermultus config %v/%v \n", namespace, spiderMultusNadName)

		// Generate Deployment yaml and annotation
		dsObject := common.GenerateExampleDaemonSetYaml(dsName, namespace)
		dsObject.Spec.Template.Annotations = map[string]string{common.MultusNetworks: fmt.Sprintf("%s/%s", namespace, spiderMultusNadName)}
		GinkgoWriter.Printf("Try to create Deployment: %v/%v \n", namespace, dsName)
		Expect(frame.CreateDaemonSet(dsObject)).NotTo(HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		defer cancel()
		err := frame.WaitPodListRunning(dsObject.Spec.Template.Labels, 2, ctx)
		Expect(err).NotTo(HaveOccurred())

		GinkgoWriter.Println("Check that each node where the Pod is located should have a vlan sub-interface.")
		checkMasterUPString := fmt.Sprintf("ip link show up %s ", common.NIC1)
		checkIPLinkString := fmt.Sprintf("ip link show up %s.%d ", common.NIC1, vlanInterface)
		Eventually(func() bool {
			for _, node := range frame.Info.KindNodeList {
				showMasterResult, err := frame.DockerExecCommand(ctx, node, checkMasterUPString)
				if err != nil {
					GinkgoWriter.Printf("Failed to execute command %s on the node %s : %v \n", checkMasterUPString, node, showMasterResult)
					return false
				}

				if string(showMasterResult) == "" {
					GinkgoWriter.Printf("master interface %s is down, waiting \n", common.NIC1)
					return false
				}

				showResult, err := frame.DockerExecCommand(ctx, node, checkIPLinkString)
				if err != nil {
					GinkgoWriter.Printf("Failed to execute %s on the node %s: %v \n", checkIPLinkString, node, showResult)
					return false
				}

				if string(showResult) == "" {
					GinkgoWriter.Printf("vlan interface %s is down, waiting... \n", vlanInterface)
					return false
				}

			}
			return true
		}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

		GinkgoWriter.Println("Create a vlan sub-interface with the same name, its network card status is down, and it is automatically set to up")
		ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		defer cancel()

		setDownString := fmt.Sprintf("ip link set %s.%d down", common.NIC1, vlanInterface)
		for _, node := range frame.Info.KindNodeList {
			_, err := frame.DockerExecCommand(ctx, node, setDownString)
			Expect(err).NotTo(HaveOccurred(), "Failed to execute  %s on the node %s:  %v", setDownString, node, err)
		}
		GinkgoWriter.Println("Restart all pods")
		podList, err := frame.GetPodListByLabel(dsObject.Spec.Template.Labels)
		Expect(err).NotTo(HaveOccurred(), "failed to get Pod list, Pod list is %v", len(podList.Items))
		Expect(frame.DeletePodList(podList)).NotTo(HaveOccurred())

		time.Sleep(time.Second * 5)

		ctx, cancel = context.WithTimeout(context.Background(), common.PodReStartTimeout)
		defer cancel()

		err = frame.WaitPodListRunning(dsObject.Spec.Template.Labels, 2, ctx)
		Expect(err).NotTo(HaveOccurred())

		GinkgoWriter.Println("Check the nic status should be up")
		checkIPLinkUpString := fmt.Sprintf("ip link show up %s.%d", common.NIC1, vlanInterface)
		Eventually(func() bool {
			for _, node := range frame.Info.KindNodeList {
				showResult, err := frame.DockerExecCommand(ctx, node, checkIPLinkUpString)
				if err != nil {
					GinkgoWriter.Printf("Failed to execute \"%s\" on the node %s: %v \n", checkIPLinkUpString, node, err)
					return false
				}

				if string(showResult) == "" {
					GinkgoWriter.Printf("vlan interface %s is down, waiting... \n", vlanInterface)
				}
			}
			return true
		}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

		// macvlan issue: https://github.com/containernetworking/plugins/pull/954
		// When the master interface on the node has been deleted, and loadConf tries
		// to get the MTU, This causes cmdDel to return a linkNotFound error to the
		// runtime.
		// TODO(cyclinder): undo the following comment if cni-plugins has new release
		//	GinkgoWriter.Println("After the host is restarted, the sub-interface is lost. Restarting the Pod will refresh the sub-interface.")
		//	ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		//	defer cancel()
		//
		//	deleteIPLinkString := fmt.Sprintf("ip link delete dev %s.%d", common.NIC1, vlanInterface)
		//	Eventually(func() bool {
		//		for _, node := range frame.Info.KindNodeList {
		//			_, err := frame.DockerExecCommand(ctx, node, deleteIPLinkString)
		//			Expect(err).NotTo(HaveOccurred(), "Failed to execute the delete sub-interface command on the node %s %v", node, err)
		//		}
		//		return true
		//	}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())
		//
		//	GinkgoWriter.Println("After deleting the sub-interface, restart all Pods")
		//	podList, err = frame.GetPodListByLabel(dsObject.Spec.Template.Labels)
		//	Expect(err).NotTo(HaveOccurred(), "failed to get Pod list, Pod list is %v", len(podList.Items))
		//	Expect(frame.DeletePodList(podList)).NotTo(HaveOccurred())
		//
		//	GinkgoWriter.Println("The sub-interfaces of all nodes are automatically rebuilt, and the status is UP")
		//	ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		//	defer cancel()
		//
		//	err = frame.WaitPodListRunning(dsObject.Spec.Template.Labels, 2, ctx)
		//	Expect(err).NotTo(HaveOccurred())
		//
		//	checkIPLinkString = fmt.Sprintf("ip link show up %s.%d", common.NIC1, vlanInterface)
		//	Eventually(func() bool {
		//		for _, node := range frame.Info.KindNodeList {
		//			showResult, err := frame.DockerExecCommand(ctx, node, checkIPLinkUpString)
		//			if err != nil {
		//				GinkgoWriter.Printf("failed to check if subinterfaces is up on node %s: %v \n", node, err)
		//				return false
		//			}
		//
		//			if len(showResult) == 0 {
		//				GinkgoWriter.Printf("sub-vlan interfaces is down, waiting...")
		//				return false
		//			}
		//		}
		//		return true
		//	}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())
	})

	// N00004: Different VLAN interfaces have the same VLAN id, an error is returned
	// N00005: The master interface is down, setting it up and creating VLAN interface
	It("Creating a VLAN interface sets the primary interface from down to up while disallowing subinterfaces with the same vlan ID.", Serial, Label("N00004", "N00005"), func() {

		mainInterface = common.NIC5
		ctx, cancel := context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		defer cancel()
		showString := fmt.Sprintf("ip link show %s", mainInterface)
		for _, node := range frame.Info.KindNodeList {
			out, err := frame.DockerExecCommand(ctx, node, showString)
			if err != nil {
				Skip(fmt.Sprintf("Node does not have additional NIC '%s', result %v, ignore this It", mainInterface, string(out)))
			}
		}

		Expect(frame.CreateSpiderMultusInstance(spiderMultusConfig)).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Create spidermultus config %v/%v \n", namespace, spiderMultusNadName)

		GinkgoWriter.Println("The master interface is down, setting it up and creating VLAN interface")
		ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		defer cancel()
		setDownString := fmt.Sprintf("ip link set %s down", mainInterface)
		Eventually(func() bool {
			for _, node := range frame.Info.KindNodeList {
				out, err := frame.DockerExecCommand(ctx, node, setDownString)
				Expect(err).NotTo(HaveOccurred(), "Executing the set sub-interface to down command on the node %s fails, error: %v, log: %v", node, err, string(out))
			}
			return true
		}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

		sameVlanInterface := 50
		GinkgoWriter.Println("Generate vlan ID of sub-interface:", sameVlanInterface)
		newSpiderMultusNadName := "new-test-multus-" + common.GenerateString(10, true)
		spiderMultusConfig = &spiderpoolv2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      newSpiderMultusNadName,
				Namespace: namespace,
			},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: pointer.String(constant.MacvlanCNI),
				MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
					Master: []string{mainInterface},
					VlanID: pointer.Int32(int32(sameVlanInterface)),
					SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
						IPv4IPPool: []string{common.SpiderPoolIPv4PoolDefault},
						IPv6IPPool: []string{common.SpiderPoolIPv6PoolDefault},
					},
				},
			},
		}
		GinkgoWriter.Printf("Generate spiderMultusConfig %v \n", spiderMultusConfig)
		Expect(frame.CreateSpiderMultusInstance(spiderMultusConfig)).NotTo(HaveOccurred())
		GinkgoWriter.Printf("Create spidermultus config %v/%v \n", namespace, spiderMultusNadName)

		dsName = "ds-1-" + common.GenerateString(10, true)
		// Generate Deployment yaml and annotation
		dsObject := common.GenerateExampleDaemonSetYaml(dsName, namespace)
		dsObject.Spec.Template.Annotations = map[string]string{common.MultusNetworks: fmt.Sprintf("%s/%s", namespace, newSpiderMultusNadName)}
		GinkgoWriter.Printf("Try to create Daemonset: %v/%v \n", namespace, dsName)
		Expect(frame.CreateDaemonSet(dsObject)).NotTo(HaveOccurred())

		ctx, cancel = context.WithTimeout(context.Background(), common.ExecCommandTimeout)
		defer cancel()
		err := frame.WaitPodListRunning(dsObject.Spec.Template.Labels, 2, ctx)
		Expect(err).NotTo(HaveOccurred())

		GinkgoWriter.Println("Check that each node where the Pod is located should have a vlan sub-interface.")
		checkMasterUPString := fmt.Sprintf("ip link show up %s ", mainInterface)
		checkIPLinkString := fmt.Sprintf("ip link show up %s.%d ", mainInterface, vlanInterface)
		Eventually(func() bool {
			for _, node := range frame.Info.KindNodeList {
				showMasterResult, err := frame.DockerExecCommand(ctx, node, checkMasterUPString)
				if err != nil {
					GinkgoWriter.Printf("Failed to execute command %s on the node %s : %v \n", checkMasterUPString, node, showMasterResult)
					return false
				}

				if string(showMasterResult) == "" {
					GinkgoWriter.Printf("master interface %s is down, waiting \n", mainInterface)
					return false
				}

				showResult, err := frame.DockerExecCommand(ctx, node, checkIPLinkString)
				if err != nil {
					GinkgoWriter.Printf("Failed to execute %s on the node %s: %v \n", checkIPLinkString, node, showResult)
					return false
				}

				if string(showResult) == "" {
					GinkgoWriter.Printf("vlan interface %s is down, waiting... \n", vlanInterface)
					return false
				}

			}
			return true
		}, common.ResourceDeleteTimeout, common.ForcedWaitingTime).Should(BeTrue())

		// Generate Deployment yaml and annotation
		dsName = "ds-2-" + common.GenerateString(10, true)
		dsObject = common.GenerateExampleDaemonSetYaml(dsName, namespace)
		dsObject.Spec.Template.Annotations = map[string]string{common.MultusNetworks: fmt.Sprintf("%s/%s", namespace, spiderMultusNadName)}
		GinkgoWriter.Printf("Try to create Daemonset: %v/%v \n", namespace, dsName)
		Expect(frame.CreateDaemonSet(dsObject)).NotTo(HaveOccurred())

		var podList *corev1.PodList
		// Wait for Pod replicas on all nodes to be pulled up.
		Eventually(func() bool {
			podList, err = frame.GetPodListByLabel(dsObject.Spec.Template.Labels)
			if err != nil {
				GinkgoWriter.Printf("failed to get pod list by label, error is %v", err)
				return false
			}
			return len(podList.Items) == len(frame.Info.KindNodeList)
		}, common.PodStartTimeout, common.ForcedWaitingTime).Should(BeTrue())

		sameVlanIdErrorString := fmt.Sprintf("cannot have multiple different vlan interfaces with the same vlanId %v on node at the same time", vlanInterface)
		for _, pod := range podList.Items {
			ctx, cancel = context.WithTimeout(context.Background(), common.EventOccurTimeout)
			defer cancel()
			err = frame.WaitExceptEventOccurred(ctx, common.OwnerPod, pod.Name, namespace, sameVlanIdErrorString)
			Expect(err).NotTo(HaveOccurred())
		}
	})
})
