// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ifacer_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

var _ = Describe("test ifacer", Label("ifacer"), func() {
	var namespace, v4PoolName, v6PoolName, dsName, spiderMultusNadName string
	var iPv4PoolObj, iPv6PoolObj *spiderpoolv2beta1.SpiderIPPool
	var v4SubnetName, v6SubnetName string
	var vlanInterface int
	var v4SubnetObject, v6SubnetObject *spiderpoolv2beta1.SpiderSubnet
	var spiderMultusConfig *spiderpoolv2beta1.SpiderMultusConfig

	BeforeEach(func() {
		dsName = "ds-" + common.GenerateString(10, true)
		namespace = "ns" + tools.RandomName()
		spiderMultusNadName = "test-multus-" + common.GenerateString(10, true)

		vlanInterface = 50
		GinkgoWriter.Println("Generate vlan ID of sub-interface:", vlanInterface)

		err := frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)
		GinkgoWriter.Printf("create namespace %v. \n", namespace)
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		defer cancel()
		if frame.Info.IpV4Enabled {
			v4PoolName, iPv4PoolObj = common.GenerateExampleIpv4poolObject(1)
			if frame.Info.SpiderSubnetEnabled {
				v4SubnetName, v4SubnetObject = common.GenerateExampleV4SubnetObject(frame, len(frame.Info.KindNodeList))
				Expect(v4SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v4SubnetObject)).NotTo(HaveOccurred())
				err = common.CreateIppoolInSpiderSubnet(ctx, frame, v4SubnetName, iPv4PoolObj, len(frame.Info.KindNodeList))
			} else {
				err = common.CreateIppool(frame, iPv4PoolObj)
			}
			Expect(err).NotTo(HaveOccurred(), "Failed to create v4 Pool %v \n", v4PoolName)
		}

		if frame.Info.IpV6Enabled {
			v6PoolName, iPv6PoolObj = common.GenerateExampleIpv6poolObject(len(frame.Info.KindNodeList))
			if frame.Info.SpiderSubnetEnabled {
				v6SubnetName, v6SubnetObject = common.GenerateExampleV6SubnetObject(frame, len(frame.Info.KindNodeList))
				Expect(v6SubnetObject).NotTo(BeNil())
				Expect(common.CreateSubnet(frame, v6SubnetObject)).NotTo(HaveOccurred())
				err = common.CreateIppoolInSpiderSubnet(ctx, frame, v6SubnetName, iPv6PoolObj, len(frame.Info.KindNodeList))
			} else {
				err = common.CreateIppool(frame, iPv6PoolObj)
			}
			Expect(err).NotTo(HaveOccurred(), "Failed to create v6 Pool %v \n", v6PoolName)
		}

		spiderMultusConfig = &spiderpoolv2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      spiderMultusNadName,
				Namespace: namespace,
			},
			Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
				CniType: "macvlan",
				MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
					Master: []string{common.NIC1},
					VlanID: ptr.To(int32(vlanInterface)),
					SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
						IPv4IPPool: []string{v4PoolName},
						IPv6IPPool: []string{v6PoolName},
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
})
