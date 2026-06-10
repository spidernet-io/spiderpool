// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package eni_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/e2eframework/tools"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

const (
	eniSlotResourceName = corev1.ResourceName(constant.DefaultENISlotResourceName)
	nodeHostnameLabel   = "kubernetes.io/hostname"
)

var testNamespace string

var _ = Describe("ENI device plugin", Label("eni-device-plugin"), Serial, func() {
	BeforeEach(func() {
		testNamespace = "eni-" + tools.RandomName()
		Expect(frame.CreateNamespaceUntilDefaultServiceAccountReady(testNamespace, common.ServiceAccountReadyTimeout)).To(Succeed())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			Expect(frame.DeleteNamespace(testNamespace)).To(Succeed())
		})
	})

	It("schedules Pods only up to advertised ENI slot capacity", Label("E00019", "US1"), func() {
		node, total := requireNodeWithENISlots()

		running := newENISlotPod("eni-capacity-holder", testNamespace, node, total)
		Expect(frame.CreatePod(running)).To(Succeed())
		waitPodRunning(running.Name)

		excess := newENISlotPod("eni-capacity-excess", testNamespace, node, 1)
		Expect(frame.CreatePod(excess)).To(Succeed())
		waitPodPendingWithoutNode(excess.Name)
	})

	It("reports node allocatable as the configured ENI slot total", Label("E00031", "US2"), func() {
		node, total := requireNodeWithENISlots()

		Consistently(func(g Gomega) {
			latest, err := frame.GetNode(node.Name)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(eniSlotQuantity(latest)).To(Equal(total))
		}).WithTimeout(30 * time.Second).WithPolling(3 * time.Second).Should(Succeed())
	})

	It("returns schedulable ENI slot capacity after Pod deletion", Label("E00043", "US3"), func() {
		node, total := requireNodeWithENISlots()

		first := newENISlotPod("eni-release-first", testNamespace, node, total)
		Expect(frame.CreatePod(first)).To(Succeed())
		first = waitPodRunning(first.Name)

		ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
		defer cancel()
		Expect(frame.DeletePodUntilFinish(first.Name, testNamespace, ctx)).To(Succeed())

		second := newENISlotPod("eni-release-second", testNamespace, node, total)
		Expect(frame.CreatePod(second)).To(Succeed())
		second = waitPodRunning(second.Name)
		Expect(second.Spec.NodeName).To(Equal(node.Name))
	})

	It("recovers ENI slot allocatable after spiderpool-agent restart", Label("E00044", "US3"), func() {
		node, total := requireNodeWithENISlots()
		agent := requireSpiderpoolAgentPodOnNode(node.Name)

		ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
		defer cancel()
		Expect(frame.DeletePodUntilFinish(agent.Name, agent.Namespace, ctx)).To(Succeed())

		Eventually(func(g Gomega) {
			pod := findSpiderpoolAgentPodOnNode(node.Name)
			g.Expect(pod).NotTo(BeNil())
			g.Expect(pod.UID).NotTo(Equal(agent.UID))
			g.Expect(pod.Status.Phase).To(Equal(corev1.PodRunning))
		}).WithTimeout(common.PodReStartTimeout).WithPolling(5 * time.Second).Should(Succeed())

		Eventually(func(g Gomega) {
			latest, err := frame.GetNode(node.Name)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(eniSlotQuantity(latest)).To(Equal(total))
		}).WithTimeout(common.PodReStartTimeout).WithPolling(5 * time.Second).Should(Succeed())
	})
})

func requireNodeWithENISlots() (*corev1.Node, int64) {
	nodes, err := frame.GetNodeList()
	Expect(err).NotTo(HaveOccurred())

	for i := range nodes.Items {
		total := eniSlotQuantity(&nodes.Items[i])
		if total > 0 {
			return &nodes.Items[i], total
		}
	}

	Skip(fmt.Sprintf("no node advertises %s; enable iaasNetworkProvider.eniDevPlugin for this e2e suite", eniSlotResourceName))
	return nil, 0
}

func eniSlotQuantity(node *corev1.Node) int64 {
	if node == nil {
		return 0
	}
	quantity, ok := node.Status.Allocatable[eniSlotResourceName]
	if !ok {
		return 0
	}
	return quantity.Value()
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

func waitPodRunning(name string) *corev1.Pod {
	ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
	defer cancel()

	pod, err := frame.WaitPodStarted(name, testNamespace, ctx)
	Expect(err).NotTo(HaveOccurred())
	Expect(pod.Spec.NodeName).NotTo(BeEmpty())
	return pod
}

func waitPodPendingWithoutNode(name string) {
	Eventually(func(g Gomega) {
		pod, err := frame.GetPod(name, testNamespace)
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
