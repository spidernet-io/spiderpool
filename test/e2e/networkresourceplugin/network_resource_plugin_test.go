// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package networkresourceplugin_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

const (
	nodeHostnameLabel  = "kubernetes.io/hostname"
	subENIResourceName = corev1.ResourceName(constant.DefaultENISlotResourceName)
	testMasterNIC      = "nrpdm0"
)

var _ = Describe("Network resource plugin", Label("networkresourceplugin", "e2e"), Serial, func() {
	var namespace string

	BeforeEach(func() {
		namespace = fmt.Sprintf("nrp-%s", common.GenerateString(12, true))
		By("create namespace " + namespace)
		Expect(frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)).To(Succeed())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}
			By("delete namespace " + namespace)
			ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			defer cancel()
			Expect(frame.DeleteNamespaceUntilFinish(namespace, ctx)).To(Succeed())
		})
	})

	It("reports configured dummy master NIC resources and schedules SMC Pods only onto matching nodes", Label("networkresourceplugin_master_nic", "US2"), func() {
		resourceName := masterNICResourceName(testMasterNIC)
		By("pick a node advertising the master NIC resource " + string(resourceName))
		node := requireNodeWithResource(resourceName)
		By("pick another node without the master NIC resource " + string(resourceName))
		other := requireNodeWithoutResource(resourceName, node.Name)

		smcName := "master-nic-" + common.GenerateString(8, true)
		By("create a Macvlan SpiderMultusConfig " + smcName + " with master " + testMasterNIC)
		Expect(frame.CreateSpiderMultusInstance(newMacvlanSpiderMultusConfig(namespace, smcName, testMasterNIC))).To(Succeed())
		By("wait for the NetworkAttachmentDefinition " + smcName + " to become ready")
		waitNetworkAttachmentReady(smcName, namespace)
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			By("delete the Macvlan SpiderMultusConfig " + smcName)
			Expect(frame.DeleteSpiderMultusInstance(namespace, smcName)).To(Succeed())
		})

		By("create a positive Pod requesting the master NIC resource on the matching node")
		scheduled := newResourcePod("master-nic-positive", namespace, nil, resourceName, smcName)
		Expect(frame.CreatePod(scheduled)).To(Succeed())
		By("wait for the positive Pod to be scheduled")
		scheduled = waitPodScheduled(scheduled.Name, namespace)
		By("verify the positive Pod landed on node " + node.Name)
		Expect(scheduled.Spec.NodeName).To(Equal(node.Name))

		By("create a negative Pod pinned via NodeSelector to a node without the master NIC resource")
		blocked := newResourcePod("master-nic-negative", namespace, other, resourceName, smcName)
		Expect(frame.CreatePod(blocked)).To(Succeed())
		By("expect the negative Pod to stay Pending without a node assignment")
		waitPodPendingWithoutNode(blocked.Name, namespace)
	})

	It("injects master NIC resources via webhook and schedules Pods only onto matching nodes", Label("networkresourceplugin_master_nic_webhook", "US2"), func() {
		resourceName := masterNICResourceName(testMasterNIC)
		By("pick a node advertising the master NIC resource " + string(resourceName))
		node := requireNodeWithResource(resourceName)
		By("pick another node without the master NIC resource " + string(resourceName))
		other := requireNodeWithoutResource(resourceName, node.Name)

		smcName := "master-nic-webhook-" + common.GenerateString(8, true)
		By("create a Macvlan SpiderMultusConfig " + smcName + " with master " + testMasterNIC)
		Expect(frame.CreateSpiderMultusInstance(newMacvlanSpiderMultusConfig(namespace, smcName, testMasterNIC))).To(Succeed())
		By("wait for the NetworkAttachmentDefinition " + smcName + " to become ready")
		waitNetworkAttachmentReady(smcName, namespace)
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			By("delete the Macvlan SpiderMultusConfig " + smcName)
			Expect(frame.DeleteSpiderMultusInstance(namespace, smcName)).To(Succeed())
		})

		By("create a positive Pod without explicit resources and let the webhook inject " + string(resourceName))
		scheduled := newWebhookInjectedPod("master-nic-webhook-positive", namespace, nil, smcName)
		Expect(frame.CreatePod(scheduled)).To(Succeed())
		By("wait for the positive Pod to be scheduled")
		scheduled = waitPodScheduled(scheduled.Name, namespace)
		By("verify the webhook injected " + string(resourceName) + " into the positive Pod")
		Expect(scheduled.Spec.Containers[0].Resources.Limits).To(HaveKey(resourceName))
		Expect(scheduled.Spec.Containers[0].Resources.Requests).To(HaveKey(resourceName))
		By("verify the positive Pod landed on node " + node.Name)
		Expect(scheduled.Spec.NodeName).To(Equal(node.Name))

		By("create a negative Pod pinned via NodeSelector to a node without the master NIC resource")
		blocked := newWebhookInjectedPod("master-nic-webhook-negative", namespace, other, smcName)
		Expect(frame.CreatePod(blocked)).To(Succeed())
		By("verify the webhook injected " + string(resourceName) + " into the negative Pod")
		Eventually(func(g Gomega) {
			pod, err := frame.GetPod(blocked.Name, namespace)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(pod.Spec.Containers[0].Resources.Limits).To(HaveKey(resourceName))
			g.Expect(pod.Spec.Containers[0].Resources.Requests).To(HaveKey(resourceName))
		}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())
		By("expect the negative Pod to stay Pending without a node assignment")
		waitPodPendingWithoutNode(blocked.Name, namespace)
	})
})

func requireNodeWithResource(resourceName corev1.ResourceName) *corev1.Node {
	nodes, err := frame.GetNodeList()
	Expect(err).NotTo(HaveOccurred())
	for i := range nodes.Items {
		if resourceValue(nodes.Items[i].Status.Allocatable, resourceName) > 0 {
			return &nodes.Items[i]
		}
	}

	Skip(fmt.Sprintf("no node advertises %s; create kind dummy NIC %s and enable spiderpoolAgent.networkResourcePlugin.resourceAdvertisement.masterNIC.rules includeInterfaces", resourceName, testMasterNIC))
	return nil
}

func requireNodeWithoutResource(resourceName corev1.ResourceName, excludeName string) *corev1.Node {
	nodes, err := frame.GetNodeList()
	Expect(err).NotTo(HaveOccurred())
	for i := range nodes.Items {
		node := &nodes.Items[i]
		if node.Name == excludeName {
			continue
		}
		if value := resourceValue(node.Status.Allocatable, resourceName); value == 0 {
			return node
		}
	}
	Skip(fmt.Sprintf("all available nodes advertise %s; kind dummy master NICs must differ between nodes", resourceName))
	return nil
}

func newResourcePod(name, namespace string, node *corev1.Node, resourceName corev1.ResourceName, smcName string) *corev1.Pod {
	quantity := resource.MustParse("1")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-" + common.GenerateString(8, true),
			Namespace: namespace,
			Annotations: map[string]string{
				common.MultusDefaultNetwork: fmt.Sprintf("%s/%s", namespace, smcName),
			},
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "samplepod",
					Image:           "alpine",
					ImagePullPolicy: "IfNotPresent",
					Command:         []string{"/bin/ash", "-c", "sleep infinity"},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							resourceName: quantity,
						},
						Requests: corev1.ResourceList{
							resourceName: quantity,
						},
					},
				},
			},
		},
	}
	if node != nil {
		hostname, ok := node.Labels[nodeHostnameLabel]
		if !ok || hostname == "" {
			Skip(fmt.Sprintf("node %s has no %s label", node.Name, nodeHostnameLabel))
		}
		pod.Spec.NodeSelector = map[string]string{nodeHostnameLabel: hostname}
	}
	return pod
}

func newWebhookInjectedPod(name, namespace string, node *corev1.Node, smcName string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name + "-" + common.GenerateString(8, true),
			Namespace: namespace,
			Annotations: map[string]string{
				common.MultusDefaultNetwork: fmt.Sprintf("%s/%s", namespace, smcName),
			},
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "samplepod",
					Image:           "alpine",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"/bin/ash", "-c", "sleep infinity"},
				},
			},
		},
	}
	if node != nil {
		hostname, ok := node.Labels[nodeHostnameLabel]
		if !ok || hostname == "" {
			Skip(fmt.Sprintf("node %s has no %s label", node.Name, nodeHostnameLabel))
		}
		pod.Spec.NodeSelector = map[string]string{nodeHostnameLabel: hostname}
	}
	return pod
}

func newMacvlanSpiderMultusConfig(namespace, name, master string) *spiderpoolv2beta1.SpiderMultusConfig {
	return &spiderpoolv2beta1.SpiderMultusConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
			CniType:           ptr.To(constant.MacvlanCNI),
			EnableCoordinator: ptr.To(false),
			MacvlanConfig: &spiderpoolv2beta1.SpiderMacvlanCniConfig{
				Master: []string{master},
			},
		},
	}
}

func waitNetworkAttachmentReady(name, namespace string) {
	Eventually(func() bool {
		_, err := frame.GetMultusInstance(name, namespace)
		if apierrors.IsNotFound(err) {
			return false
		}
		Expect(err).NotTo(HaveOccurred())
		return true
	}).WithTimeout(common.ResourceDeleteTimeout).WithPolling(time.Second).Should(BeTrue())
}

func waitPodScheduled(name, namespace string) *corev1.Pod {
	var pod *corev1.Pod
	Eventually(func(g Gomega) {
		var err error
		pod, err = frame.GetPod(name, namespace)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pod.Spec.NodeName).NotTo(BeEmpty())
	}).WithTimeout(common.PodStartTimeout).WithPolling(time.Second).Should(Succeed())
	return pod
}

func waitPodPendingWithoutNode(name, namespace string) {
	Eventually(func(g Gomega) {
		pod, err := frame.GetPod(name, namespace)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(pod.Status.Phase).To(Equal(corev1.PodPending))
		g.Expect(pod.Spec.NodeName).To(BeEmpty())
	}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())
}

func resourceValue(resources corev1.ResourceList, resourceName corev1.ResourceName) int64 {
	value, _ := resourceValueIfPresent(resources, resourceName)
	return value
}

func resourceValueIfPresent(resources corev1.ResourceList, resourceName corev1.ResourceName) (int64, bool) {
	quantity, ok := resources[resourceName]
	if !ok {
		return 0, false
	}
	return quantity.Value(), true
}

func masterNICResourceName(master string) corev1.ResourceName {
	return corev1.ResourceName(constant.SpiderpoolResourceDomain + "/" + master + constant.MasterNICResourceSuffix)
}
