// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package iaasnetworkprovider_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
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
	eniSlotResourceName = corev1.ResourceName(constant.DefaultENISlotResourceName)
	nodeHostnameLabel   = "kubernetes.io/hostname"
)

var _ = Describe("IaaS network provider Pod lifecycle", Label("iaasnetworkprovider"), Serial, func() {
	var namespace string

	BeforeEach(func() {
		namespace = newCaseNamespace("iaas-provider")
		Expect(frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)).To(Succeed())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			deleteNamespaceUntilFinish(namespace)
		})
	})

	It("allocates from provider for a Pod using VLAN SpiderMultusConfig and releases on deletion", Label("I00001", "US1"), func() {
		expectedSlots := expectedENISlotsPerNode()
		Expect(expectedSlots).To(Equal(int64(2)), "this case starts 3 Pods and expects exactly 1 unschedulable Pod")
		node := requireNodeWithExpectedENISlots(expectedSlots)

		poolName, pool := common.GenerateExampleIpv4poolObject(5)
		Expect(common.CreateIppool(frame, pool)).To(Succeed())
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			Expect(common.DeleteIPPoolByName(frame, poolName)).To(Succeed())
		})

		smcName := "vlan-provider-" + common.GenerateString(10, true)
		smc := newVlanSpiderMultusConfig(namespace, smcName, poolName)
		Expect(frame.CreateSpiderMultusInstance(smc)).To(Succeed())
		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				return
			}
			Expect(frame.DeleteSpiderMultusInstance(namespace, smcName)).To(Succeed())
		})
		waitNetworkAttachmentReady(smcName, namespace)

		Expect(providerMock.Reset()).To(Succeed())

		podNames := []string{
			"provider-pod-1-" + common.GenerateString(8, true),
			"provider-pod-2-" + common.GenerateString(8, true),
			"provider-pod-3-" + common.GenerateString(8, true),
		}
		for _, podName := range podNames {
			pod := newProviderPod(podName, namespace, smcName, node)
			GinkgoWriter.Printf("create provider Pod %s/%s with default network %s/%s on node %s\n", namespace, podName, namespace, smcName, node.Name)
			Expect(frame.CreatePod(pod)).To(Succeed())
		}

		expectPodsInjectedENISlotResource(podNames, namespace, 1)
		runningPods, pendingPod := waitPodsSchedulingResult(podNames, namespace, int(expectedSlots), 1)
		expectFailedSchedulingEvent(pendingPod)
		for i := range runningPods {
			expectProviderCall(providerMockAllocatePath, runningPods[i].Name, namespace)
			expectSpiderEndpointMatchesProviderCache(&runningPods[i])
		}

		ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
		GinkgoWriter.Printf("delete pending provider Pod %s/%s before releasing any running Pod slot\n", namespace, pendingPod.Name)
		Expect(frame.DeletePodUntilFinish(pendingPod.Name, namespace, ctx)).To(Succeed())
		cancel()

		for i := range runningPods {
			ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
			GinkgoWriter.Printf("delete running provider Pod %s/%s and expect release call\n", namespace, runningPods[i].Name)
			Expect(frame.DeletePodUntilFinish(runningPods[i].Name, namespace, ctx)).To(Succeed())
			cancel()
			expectProviderCall(providerMockReleasePath, runningPods[i].Name, namespace)
		}
	})
})

func newVlanSpiderMultusConfig(namespace, name, ipv4Pool string) *spiderpoolv2beta1.SpiderMultusConfig {
	return &spiderpoolv2beta1.SpiderMultusConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: spiderpoolv2beta1.MultusCNIConfigSpec{
			CniType: ptr.To(constant.VlanCNI),
			// This case validates IaaS provider allocation, not coordinator route tuning.
			// Disable coordinator so the generated NAD only exercises VLAN + Spiderpool IPAM.
			EnableCoordinator: ptr.To(false),
			VlanConfig: &spiderpoolv2beta1.SpiderVlanCniConfig{
				Master: []string{common.NIC1},
				SpiderpoolConfigPools: &spiderpoolv2beta1.SpiderpoolPools{
					IPv4IPPool: []string{ipv4Pool},
				},
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

func newProviderPod(name, namespace, smcName string, node *corev1.Node) *corev1.Pod {
	Expect(node).NotTo(BeNil())
	hostname := node.Labels[nodeHostnameLabel]
	Expect(hostname).NotTo(BeEmpty(), "node %s has no %s label", node.Name, nodeHostnameLabel)

	pod := common.GenerateExamplePodYaml(name, namespace)
	pod.Annotations[common.MultusDefaultNetwork] = fmt.Sprintf("%s/%s", namespace, smcName)
	pod.Spec.NodeSelector = map[string]string{
		nodeHostnameLabel: hostname,
	}
	return pod
}

func expectedENISlotsPerNode() int64 {
	value := os.Getenv("E2E_IAAS_PROVIDER_ENI_MAX_SLOTS_PER_NODE")
	if value == "" {
		return 2
	}
	slots, err := strconv.ParseInt(value, 10, 64)
	Expect(err).NotTo(HaveOccurred(), "invalid E2E_IAAS_PROVIDER_ENI_MAX_SLOTS_PER_NODE=%q", value)
	Expect(slots).To(BeNumerically(">", 0), "E2E_IAAS_PROVIDER_ENI_MAX_SLOTS_PER_NODE must be greater than 0")
	return slots
}

func requireNodeWithExpectedENISlots(expected int64) *corev1.Node {
	nodes, err := frame.GetNodeList()
	Expect(err).NotTo(HaveOccurred())

	for i := range nodes.Items {
		node := &nodes.Items[i]
		capacity := eniSlotQuantity(node.Status.Capacity)
		allocatable := eniSlotQuantity(node.Status.Allocatable)
		GinkgoWriter.Printf("node %s ENI slot capacity=%d allocatable=%d expected=%d\n", node.Name, capacity, allocatable, expected)
		if capacity == 0 && allocatable == 0 {
			continue
		}
		Expect(capacity).To(Equal(expected), "node %s status.capacity[%s] mismatch", node.Name, eniSlotResourceName)
		Expect(allocatable).To(Equal(expected), "node %s status.allocatable[%s] mismatch", node.Name, eniSlotResourceName)
		if node.Labels[nodeHostnameLabel] == "" {
			Skip(fmt.Sprintf("node %s has no %s label", node.Name, nodeHostnameLabel))
		}
		return node
	}

	Fail(fmt.Sprintf("no node status advertises %s; run setup_spiderpool with E2E_IAAS_NETWORK_PROVIDER_ENABLED=true", eniSlotResourceName))
	return nil
}

func eniSlotQuantity(resources corev1.ResourceList) int64 {
	quantity, ok := resources[eniSlotResourceName]
	if !ok {
		return 0
	}
	return quantity.Value()
}

func expectPodsInjectedENISlotResource(names []string, namespace string, slots int64) {
	Eventually(func(g Gomega) {
		for _, name := range names {
			pod, err := frame.GetPod(name, namespace)
			g.Expect(err).NotTo(HaveOccurred())
			expectInjectedENISlotResource(g, pod, slots)
		}
	}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())
}

func expectInjectedENISlotResource(g Gomega, pod *corev1.Pod, slots int64) {
	g.Expect(pod.Spec.Containers).NotTo(BeEmpty())
	quantity := *resource.NewQuantity(slots, resource.DecimalSI)
	container := pod.Spec.Containers[0]
	GinkgoWriter.Printf("Pod %s/%s ENI slot resources: requests=%v limits=%v\n", pod.Namespace, pod.Name, container.Resources.Requests, container.Resources.Limits)
	request, ok := container.Resources.Requests[eniSlotResourceName]
	g.Expect(ok).To(BeTrue(), "Pod %s/%s requests[%s] missing", pod.Namespace, pod.Name, eniSlotResourceName)
	g.Expect(request.Cmp(quantity)).To(Equal(0), "Pod %s/%s requests[%s] mismatch", pod.Namespace, pod.Name, eniSlotResourceName)
	limit, ok := container.Resources.Limits[eniSlotResourceName]
	g.Expect(ok).To(BeTrue(), "Pod %s/%s limits[%s] missing", pod.Namespace, pod.Name, eniSlotResourceName)
	g.Expect(limit.Cmp(quantity)).To(Equal(0), "Pod %s/%s limits[%s] mismatch", pod.Namespace, pod.Name, eniSlotResourceName)
}

func waitPodsSchedulingResult(names []string, namespace string, expectedRunning, expectedPending int) ([]corev1.Pod, *corev1.Pod) {
	var runningPods []corev1.Pod
	var pendingPod *corev1.Pod
	Eventually(func(g Gomega) {
		runningPods = nil
		pendingPod = nil
		pendingWithoutNode := 0
		for _, name := range names {
			pod, err := frame.GetPod(name, namespace)
			g.Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("Pod %s/%s phase=%s node=%s\n", namespace, name, pod.Status.Phase, pod.Spec.NodeName)
			switch pod.Status.Phase {
			case corev1.PodRunning:
				g.Expect(pod.Spec.NodeName).NotTo(BeEmpty())
				runningPods = append(runningPods, *pod)
			case corev1.PodPending:
				if pod.Spec.NodeName == "" {
					pendingWithoutNode++
					pendingPod = pod.DeepCopy()
				}
			}
		}
		g.Expect(runningPods).To(HaveLen(expectedRunning))
		g.Expect(pendingWithoutNode).To(Equal(expectedPending))
		g.Expect(pendingPod).NotTo(BeNil())
	}).WithTimeout(common.PodStartTimeout).WithPolling(3 * time.Second).Should(Succeed())

	Expect(runningPods).To(HaveLen(expectedRunning))
	Expect(pendingPod).NotTo(BeNil())
	return runningPods, pendingPod
}

func expectFailedSchedulingEvent(pod *corev1.Pod) {
	Expect(pod).NotTo(BeNil())
	Eventually(func(g Gomega) {
		events, err := frame.GetEvents(context.Background(), "Pod", pod.Name, pod.Namespace)
		g.Expect(err).NotTo(HaveOccurred())
		for i := range events.Items {
			event := events.Items[i]
			GinkgoWriter.Printf("Pod %s/%s event reason=%s message=%s\n", pod.Namespace, pod.Name, event.Reason, event.Message)
			if event.Reason == "FailedScheduling" && strings.Contains(event.Message, string(eniSlotResourceName)) {
				return
			}
		}
		g.Expect(false).To(BeTrue(), "no FailedScheduling event mentioning %s found for Pod %s/%s", eniSlotResourceName, pod.Namespace, pod.Name)
	}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())
}

func expectProviderCall(path, podName, namespace string) {
	Eventually(func(g Gomega) {
		records, err := providerMock.Records()
		g.Expect(err).NotTo(HaveOccurred())
		GinkgoWriter.Printf("provider mock records while waiting for %s Pod %s/%s: %+v\n", path, namespace, podName, records.Records)
		record := records.find(path, podName, namespace)
		g.Expect(record).NotTo(BeNil())
		g.Expect(record.Body["podName"]).To(Equal(podName))
		g.Expect(record.Body["podNamespace"]).To(Equal(namespace))
	}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())
}

func expectSpiderEndpointMatchesProviderCache(pod *corev1.Pod) {
	Expect(pod).NotTo(BeNil())
	Eventually(func(g Gomega) {
		endpoint, err := common.GetWorkloadByName(frame, pod.Namespace, pod.Name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(endpoint.Status.Current.IPs).NotTo(BeEmpty())
		for i := range endpoint.Status.Current.IPs {
			detail := endpoint.Status.Current.IPs[i]
			if detail.IPv4 == nil {
				continue
			}
			ipAddress := normalizeIPAddress(*detail.IPv4)
			cache, err := providerMock.IPCache(ipAddress)
			g.Expect(err).NotTo(HaveOccurred())
			GinkgoWriter.Printf("compare SpiderEndpoint %s/%s interface=%s ip=%s mac=%v vlan=%v with provider cache=%+v\n", pod.Namespace, pod.Name, detail.NIC, ipAddress, detail.MAC, detail.Vlan, cache)
			g.Expect(cache.IPAddress).To(Equal(ipAddress))
			g.Expect(cache.NodeName).To(Equal(pod.Spec.NodeName))
			g.Expect(detail.MAC).NotTo(BeNil())
			g.Expect(*detail.MAC).To(Equal(cache.Mac))
			g.Expect(detail.Vlan).NotTo(BeNil())
			g.Expect(*detail.Vlan).To(Equal(cache.VlanID))
			return
		}
		g.Expect(false).To(BeTrue(), "SpiderEndpoint %s/%s has no IPv4 allocation detail", pod.Namespace, pod.Name)
	}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())
}

func normalizeIPAddress(address string) string {
	ip, _, err := net.ParseCIDR(address)
	if err == nil {
		return ip.String()
	}
	parsed := net.ParseIP(address)
	Expect(parsed).NotTo(BeNil(), "invalid IP address %q", address)
	return parsed.String()
}

func (r *providerMockRecords) find(path, podName, namespace string) *providerMockRecord {
	if r == nil {
		return nil
	}
	for i := len(r.Records) - 1; i >= 0; i-- {
		if r.Records[i].Path == path &&
			r.Records[i].Body["podName"] == podName &&
			r.Records[i].Body["podNamespace"] == namespace {
			return &r.Records[i]
		}
	}
	return nil
}
