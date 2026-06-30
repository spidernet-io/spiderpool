// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package iaasnetworkprovider_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

var _ = Describe("ENI device plugin", Label("iaasnetworkprovider", "eni-device-plugin"), Serial, func() {
	var namespace string

	BeforeEach(func() {
		namespace = newCaseNamespace("eni")
		By("create namespace " + namespace)
		Expect(frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)).To(Succeed())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			By("delete namespace " + namespace)
			deleteNamespaceUntilFinish(namespace)
		})
	})

	It("schedules Pods only up to advertised ENI slot capacity", Label("E00019", "US1"), func() {
		By("pick a node advertising ENI slot capacity")
		node, total := requireNodeWithENISlotsForDevicePlugin()

		By("create a capacity-holder Pod requesting all " + fmt.Sprintf("%d", total) + " ENI slots on node " + node.Name)
		running := newENISlotPod("eni-capacity-holder", namespace, node, total)
		Expect(frame.CreatePod(running)).To(Succeed())
		By("wait for the capacity-holder Pod to run on node " + node.Name)
		waitENISlotPodRunning(running.Name, namespace)

		By("create an excess Pod requesting 1 more ENI slot on the same node")
		excess := newENISlotPod("eni-capacity-excess", namespace, node, 1)
		Expect(frame.CreatePod(excess)).To(Succeed())
		By("expect the excess Pod to stay Pending without a node assignment")
		waitENISlotPodPendingWithoutNode(excess.Name, namespace)
	})

	It("reports node allocatable as the configured ENI slot total", Label("E00031", "US2"), func() {
		By("pick a node advertising ENI slot capacity")
		node, total := requireNodeWithENISlotsForDevicePlugin()

		By("consistently verify node " + node.Name + " allocatable ENI slots equal " + fmt.Sprintf("%d", total))
		Consistently(func(g Gomega) {
			latest, err := frame.GetNode(node.Name)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(eniSlotQuantity(latest.Status.Allocatable)).To(Equal(total))
		}).WithTimeout(30 * time.Second).WithPolling(3 * time.Second).Should(Succeed())
	})

	It("returns schedulable ENI slot capacity after Pod deletion", Label("E00043", "US3"), func() {
		By("pick a node advertising ENI slot capacity")
		node, total := requireNodeWithENISlotsForDevicePlugin()

		By("create a first Pod requesting all " + fmt.Sprintf("%d", total) + " ENI slots on node " + node.Name)
		first := newENISlotPod("eni-release-first", namespace, node, total)
		Expect(frame.CreatePod(first)).To(Succeed())
		first = waitENISlotPodRunning(first.Name, namespace)

		By("delete the first Pod to free the ENI slots on node " + node.Name)
		ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
		defer cancel()
		Expect(frame.DeletePodUntilFinish(first.Name, namespace, ctx)).To(Succeed())

		By("create a second Pod requesting the freed ENI slots on the same node")
		second := newENISlotPod("eni-release-second", namespace, node, total)
		Expect(frame.CreatePod(second)).To(Succeed())
		second = waitENISlotPodRunning(second.Name, namespace)
		By("verify the second Pod is scheduled on node " + node.Name)
		Expect(second.Spec.NodeName).To(Equal(node.Name))
	})

	It("recovers ENI slot allocatable after spiderpool-agent restart", Label("E00044", "US3"), func() {
		By("pick a node advertising ENI slot capacity")
		node, total := requireNodeWithENISlotsForDevicePlugin()
		By("locate the spiderpool-agent Pod running on node " + node.Name)
		agent := requireSpiderpoolAgentPodOnNode(node.Name)

		By("delete the spiderpool-agent Pod " + agent.Namespace + "/" + agent.Name + " to trigger a restart")
		ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
		defer cancel()
		Expect(frame.DeletePodUntilFinish(agent.Name, agent.Namespace, ctx)).To(Succeed())

		By("wait for a replacement spiderpool-agent Pod to run on node " + node.Name)
		Eventually(func(g Gomega) {
			pod := findSpiderpoolAgentPodOnNode(node.Name)
			g.Expect(pod).NotTo(BeNil())
			g.Expect(pod.UID).NotTo(Equal(agent.UID))
			g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
		}).WithTimeout(common.PodReStartTimeout).WithPolling(5 * time.Second).Should(Succeed())

		By("verify node " + node.Name + " ENI slot allocatable recovers to " + fmt.Sprintf("%d", total))
		Eventually(func(g Gomega) {
			latest, err := frame.GetNode(node.Name)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(eniSlotQuantity(latest.Status.Allocatable)).To(Equal(total))
		}).WithTimeout(common.PodReStartTimeout).WithPolling(5 * time.Second).Should(Succeed())
	})

	It("blocks webhook-injected sub-eni Pods when advertised capacity is exhausted", Label("E00020", "US1"), func() {
		By("pick a node advertising ENI slot capacity")
		node, total := requireNodeWithENISlotsForDevicePlugin()

		By("create a capacity-holder Pod requesting all " + fmt.Sprintf("%d", total) + " ENI slots on node " + node.Name)
		holder := newENISlotPod("eni-webhook-holder", namespace, node, total)
		Expect(frame.CreatePod(holder)).To(Succeed())
		By("wait for the capacity-holder Pod to run on node " + node.Name)
		waitENISlotPodRunning(holder.Name, namespace)
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel()
			Expect(frame.DeletePodUntilFinish(holder.Name, namespace, ctx)).To(Succeed())
		})

		poolName, pool := common.GenerateExampleIpv4poolObject(5)
		By("create an IPv4 IPPool " + poolName)
		Expect(common.CreateIppool(frame, pool)).To(Succeed())
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			Expect(common.DeleteIPPoolByName(frame, poolName)).To(Succeed())
		})

		smcName := "vlan-webhook-excess-" + common.GenerateString(8, true)
		By("create a VLAN SpiderMultusConfig " + smcName + " with vlanMode auto")
		Expect(frame.CreateSpiderMultusInstance(newVlanSpiderMultusConfig(namespace, smcName, poolName))).To(Succeed())
		By("wait for the NetworkAttachmentDefinition " + smcName + " to become ready")
		waitNetworkAttachmentReady(smcName, namespace)
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			Expect(frame.DeleteSpiderMultusInstance(namespace, smcName)).To(Succeed())
		})

		By("create a Pod without explicit resources referencing the VLAN auto SMC on the same node")
		pod := newProviderPod("eni-webhook-excess", namespace, smcName, node)
		Expect(frame.CreatePod(pod)).To(Succeed())

		By("verify the webhook injected sub-eni resource into the Pod")
		Eventually(func(g Gomega) {
			latest, err := frame.GetPod(pod.Name, namespace)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(latest.Spec.Containers[0].Resources.Limits).To(HaveKey(eniSlotResourceName))
			g.Expect(latest.Spec.Containers[0].Resources.Requests).To(HaveKey(eniSlotResourceName))
		}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())

		By("expect the Pod to stay Pending without a node assignment due to insufficient sub-eni")
		waitENISlotPodPendingWithoutNode(pod.Name, namespace)
	})

	It("injects both sub-eni and master NIC resources via webhook for a VLAN auto SpiderMultusConfig", Label("E00021", "US1", "US2"), func() {
		By("pick a node advertising both ENI slot and master NIC capacity")
		node, master := requireNodeWithENISlotsAndMasterNIC()
		masterResource := masterNICResourceNameFromMaster(master)

		poolName, pool := common.GenerateExampleIpv4poolObject(5)
		By("create an IPv4 IPPool " + poolName)
		Expect(common.CreateIppool(frame, pool)).To(Succeed())
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			Expect(common.DeleteIPPoolByName(frame, poolName)).To(Succeed())
		})

		smcName := "vlan-combined-webhook-" + common.GenerateString(8, true)
		By("create a VLAN SpiderMultusConfig " + smcName + " with master " + master + " and vlanMode auto")
		Expect(frame.CreateSpiderMultusInstance(newVlanSpiderMultusConfigWithMaster(namespace, smcName, poolName, master))).To(Succeed())
		By("wait for the NetworkAttachmentDefinition " + smcName + " to become ready")
		waitNetworkAttachmentReady(smcName, namespace)
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			Expect(frame.DeleteSpiderMultusInstance(namespace, smcName)).To(Succeed())
		})

		By("create a Pod without explicit resources referencing the VLAN auto SMC on node " + node.Name)
		pod := newProviderPod("eni-combined-webhook", namespace, smcName, node)
		Expect(frame.CreatePod(pod)).To(Succeed())

		By("verify the webhook injected both sub-eni and master NIC resources")
		Eventually(func(g Gomega) {
			latest, err := frame.GetPod(pod.Name, namespace)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(latest.Spec.Containers[0].Resources.Limits).To(HaveKey(eniSlotResourceName))
			g.Expect(latest.Spec.Containers[0].Resources.Requests).To(HaveKey(eniSlotResourceName))
			g.Expect(latest.Spec.Containers[0].Resources.Limits).To(HaveKey(masterResource))
			g.Expect(latest.Spec.Containers[0].Resources.Requests).To(HaveKey(masterResource))
		}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())

		By("wait for the Pod to be scheduled on node " + node.Name)
		scheduled := waitENISlotPodRunning(pod.Name, namespace)
		Expect(scheduled.Spec.NodeName).To(Equal(node.Name))
	})
})

func requireNodeWithENISlotsForDevicePlugin() (*corev1.Node, int64) {
	nodes, err := frame.GetNodeList()
	Expect(err).NotTo(HaveOccurred())

	for i := range nodes.Items {
		total := eniSlotQuantity(nodes.Items[i].Status.Allocatable)
		if total > 0 {
			return &nodes.Items[i], total
		}
	}

	Skip(fmt.Sprintf("no node advertises %s; enable spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.subENI for this e2e suite", eniSlotResourceName))
	return nil, 0
}

func newENISlotPod(name, namespace string, node *corev1.Node, slots int64) *corev1.Pod {
	Expect(node).NotTo(BeNil())
	Expect(slots).To(BeNumerically(">", 0))
	hostname, ok := node.Labels[nodeHostnameLabel]
	if !ok || hostname == "" {
		Skip(fmt.Sprintf("node %s has no %s label", node.Name, nodeHostnameLabel))
	}

	quantity := resource.NewQuantity(slots, resource.DecimalSI)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.PodSpec{
			NodeSelector: map[string]string{
				nodeHostnameLabel: hostname,
			},
			Containers: []corev1.Container{
				{
					Name:            "samplepod",
					Image:           "alpine",
					ImagePullPolicy: "IfNotPresent",
					Command:         []string{"/bin/ash", "-c", "sleep infinity"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							eniSlotResourceName: *quantity,
						},
						Requests: corev1.ResourceList{
							eniSlotResourceName: *quantity,
						},
					},
				},
			},
		},
	}
}

func waitENISlotPodRunning(name, namespace string) *corev1.Pod {
	ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
	defer cancel()

	pod, err := frame.WaitPodStarted(name, namespace, ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(pod.Spec.NodeName).NotTo(BeEmpty())
	return pod
}

func waitENISlotPodPendingWithoutNode(name, namespace string) {
	Eventually(func(g Gomega) {
		pod, err := frame.GetPod(name, namespace)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pod.Status.Phase).To(Equal(corev1.PodPending))
		g.Expect(pod.Spec.NodeName).To(BeEmpty())
	}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())
}

func requireSpiderpoolAgentPodOnNode(nodeName string) *corev1.Pod {
	pod := findSpiderpoolAgentPodOnNode(nodeName)
	if pod != nil {
		return pod
	}

	pods, err := frame.GetPodList(
		client.MatchingLabels(map[string]string{
			"app.kubernetes.io/component": constant.SpiderpoolAgent,
		}),
	)
	Expect(err).NotTo(HaveOccurred())
	if len(pods.Items) == 0 {
		Skip("no spiderpool-agent Pods found")
	}

	Skip(fmt.Sprintf("no running spiderpool-agent Pod found on node %s", nodeName))
	return nil
}

func findSpiderpoolAgentPodOnNode(nodeName string) *corev1.Pod {
	pods, err := frame.GetPodList(
		client.MatchingLabels(map[string]string{
			"app.kubernetes.io/component": constant.SpiderpoolAgent,
		}),
	)
	Expect(err).NotTo(HaveOccurred())

	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.Spec.NodeName == nodeName && pod.Status.Phase == corev1.PodRunning {
			return pod
		}
	}
	return nil
}

func requireNodeWithENISlotsAndMasterNIC() (*corev1.Node, string) {
	nodes, err := frame.GetNodeList()
	Expect(err).NotTo(HaveOccurred())

	prefix := constant.SpiderpoolResourceDomain + "/"
	suffix := constant.MasterNICResourceSuffix
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if eniSlotQuantity(node.Status.Allocatable) == 0 {
			continue
		}
		if node.Labels[nodeHostnameLabel] == "" {
			continue
		}
		for k, v := range node.Status.Allocatable {
			s := string(k)
			if !strings.HasPrefix(s, prefix) || !strings.HasSuffix(s, suffix) || v.Value() <= 0 {
				continue
			}
			master := strings.TrimSuffix(strings.TrimPrefix(s, prefix), suffix)
			if master == "" {
				continue
			}
			return node, master
		}
	}

	Skip("no node advertises both sub-eni and a master NIC resource")
	return nil, ""
}

func masterNICResourceNameFromMaster(master string) corev1.ResourceName {
	return corev1.ResourceName(constant.SpiderpoolResourceDomain + "/" + master + constant.MasterNICResourceSuffix)
}

func newVlanSpiderMultusConfigWithMaster(namespace, name, ipv4Pool, master string) *spiderpoolv2beta1.SpiderMultusConfig {
	return &spiderpoolv2beta1.SpiderMultusConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
			CniType:           ptr.To(constant.VlanCNI),
			EnableCoordinator: ptr.To(false),
			VlanConfig: &spiderpoolv2beta1.SpiderVlanCniConfig{
				Master:   []string{master},
				VlanMode: ptr.To(constant.VlanModeAuto),
				SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
					IPv4IPPool: []string{ipv4Pool},
				},
			},
		},
	}
}
