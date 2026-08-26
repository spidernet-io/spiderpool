// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package iaasnetworkprovider_test

import (
	"context"
	"encoding/json"
	"net"
	"sort"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/test/e2e/common"
)

const (
	prewarmEntryMAC  = "0a:1b:2c:3d:4e:5f"
	prewarmEntryVLAN = int32(2101)
)

var _ = Describe("IaaS prewarm, global and paired pool allocation", Label("iaasnetworkprovider"), Serial, func() {
	var namespace string
	var node *corev1.Node
	var master string

	BeforeEach(func() {
		namespace = newCaseNamespace("iaas-prewarm")
		By("create namespace " + namespace)
		Expect(frame.CreateNamespaceUntilDefaultServiceAccountReady(namespace, common.ServiceAccountReadyTimeout)).To(Succeed())

		By("pick a node with enough provider network resources")
		node, master = requireNodeWithExpectedProviderResources(expectedENISlotsPerNode())

		By("reset the IaaS provider mock server record store")
		Expect(providerMock.Reset()).To(Succeed())

		DeferCleanup(func() {
			if CurrentSpecReport().Failed() {
				GinkgoWriter.Println("If the use case fails, the cleanup step will be skipped")
				return
			}

			By("delete namespace " + namespace)
			deleteNamespaceUntilFinish(namespace)
		})
	})

	It("allocates and releases from a node-level prewarm pool without any provider RPC", Label("I00025"), func() {
		v4PoolName, v6PoolName := createIaaSPoolPerFamily(func(pool *spiderpoolv2beta1.SpiderIPPool) {
			markIaaSProviderPool(pool)
			pool.Spec.NodeName = []string{node.Name}
		})

		By("simulate the provider prewarm flush: write node-scoped metadata entries for every pool IP")
		writePoolMetadata(v4PoolName, node.Name, master, nodeLevelEntriesForPool(v4PoolName))
		writePoolMetadata(v6PoolName, node.Name, master, nodeLevelEntriesForPool(v6PoolName))

		smcName := setupProviderNetwork(namespace, v4PoolName, v6PoolName, master)

		pod := startProviderPod(namespace, smcName, node)

		By("verify the allocation was served from prewarm metadata with zero provider RPC")
		expectNoProviderCall(providerMockAllocatePath, pod.Name, namespace)

		By("verify the SpiderEndpoint carries the MAC/VLAN of the prewarm metadata entry")
		detail := endpointFirstIPDetail(pod)
		Expect(detail.MAC).NotTo(BeNil())
		Expect(*detail.MAC).To(Equal(prewarmEntryMAC))
		Expect(detail.Vlan).NotTo(BeNil())
		Expect(*detail.Vlan).To(Equal(int64(prewarmEntryVLAN)))
		Expect(detail.IPv4).NotTo(BeNil())
		Expect(detail.IPv6).NotTo(BeNil())

		deleteProviderPod(pod)

		By("verify the release kept the cloud-side reservation: zero provider release RPC")
		expectNoProviderCall(providerMockReleasePath, pod.Name, namespace)
	})

	It("gates allocation on provider metadata and recovers once the metadata is written", Label("I00026"), func() {
		v4PoolName, v6PoolName := createIaaSPoolPerFamily(func(pool *spiderpoolv2beta1.SpiderIPPool) {
			markIaaSProviderPool(pool)
			pool.Spec.NodeName = []string{node.Name}
		})

		smcName := setupProviderNetwork(namespace, v4PoolName, v6PoolName, master)

		podName := "provider-pod-" + common.GenerateString(8, true)
		pod := newProviderPod(podName, namespace, smcName, node)
		By("create the provider Pod " + namespace + "/" + podName + " before any metadata exists")
		Expect(frame.CreatePod(pod)).To(Succeed())

		By("verify the Pod stays pending: the pool metadata is not ready and no provider RPC happens")
		Consistently(func(g Gomega) {
			current, err := frame.GetPod(podName, namespace)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(current.Status.Phase).NotTo(Equal(corev1.PodRunning))

			records, err := providerMock.Records()
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(records.find(providerMockAllocatePath, podName, namespace)).To(BeNil())
		}).WithTimeout(30 * time.Second).WithPolling(3 * time.Second).Should(Succeed())

		By("simulate the provider prewarm flush: write node-scoped metadata entries for every pool IP")
		writePoolMetadata(v4PoolName, node.Name, master, nodeLevelEntriesForPool(v4PoolName))
		writePoolMetadata(v6PoolName, node.Name, master, nodeLevelEntriesForPool(v6PoolName))

		By("wait for the Pod to recover and start running from the prewarm cache")
		ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
		runningPod, err := frame.WaitPodStarted(podName, namespace, ctx)
		cancel()
		Expect(err).NotTo(HaveOccurred())

		By("verify the recovered allocation was still a zero-RPC prewarm cache hit")
		expectNoProviderCall(providerMockAllocatePath, runningPod.Name, namespace)

		deleteProviderPod(runningPod)
	})

	It("serves a global pool cold path via provider RPC and then reuses the bound entry as a sticky zero-RPC cache hit", Label("I00027"), func() {
		v4PoolName, v6PoolName := createIaaSPoolPerFamily(markGlobalIaaSPool)

		By("write the global metadata skeleton (empty scope, no entries) simulating the provider post-creation flush")
		writePoolMetadata(v4PoolName, "", master, nil)
		writePoolMetadata(v6PoolName, "", master, nil)

		smcName := setupProviderNetwork(namespace, v4PoolName, v6PoolName, master)

		By("create the first Pod: an empty entry set must take the cold path through the provider RPC")
		pod := startProviderPod(namespace, smcName, node)
		expectProviderCall(providerMockAllocatePath, pod.Name, namespace)

		detail := endpointFirstIPDetail(pod)
		Expect(detail.IPv4).NotTo(BeNil())
		Expect(detail.IPv6).NotTo(BeNil())
		firstV4 := normalizeIPAddress(*detail.IPv4)
		firstV6 := normalizeIPAddress(*detail.IPv6)

		By("simulate the provider async flush: bind the allocated addresses to node " + node.Name)
		writePoolMetadata(v4PoolName, "", master, map[string]spiderpoolv2beta1.IPMetadataEntry{
			firstV4: {MAC: prewarmEntryMAC, VLAN: ptr.To(prewarmEntryVLAN), Node: ptr.To(node.Name)},
		})
		writePoolMetadata(v6PoolName, "", master, map[string]spiderpoolv2beta1.IPMetadataEntry{
			firstV6: {MAC: prewarmEntryMAC, VLAN: ptr.To(prewarmEntryVLAN), Node: ptr.To(node.Name)},
		})

		deleteProviderPod(pod)
		By("verify the release kept the cloud-side reservation: zero provider release RPC")
		expectNoProviderCall(providerMockReleasePath, pod.Name, namespace)

		By("reset the mock and create a second Pod on the same node: the bound entry must be a sticky zero-RPC cache hit")
		Expect(providerMock.Reset()).To(Succeed())
		secondPod := startProviderPod(namespace, smcName, node)
		expectNoProviderCall(providerMockAllocatePath, secondPod.Name, namespace)

		secondDetail := endpointFirstIPDetail(secondPod)
		Expect(secondDetail.IPv4).NotTo(BeNil())
		Expect(normalizeIPAddress(*secondDetail.IPv4)).To(Equal(firstV4), "sticky global-pool reuse must hand out the same IPv4")
		Expect(secondDetail.IPv6).NotTo(BeNil())
		Expect(normalizeIPAddress(*secondDetail.IPv6)).To(Equal(firstV6), "sticky global-pool reuse must hand out the same IPv6")

		deleteProviderPod(secondPod)
	})

	It("steals a global pool entry bound to another node through the cold path provider RPC", Label("I00028"), func() {
		v4PoolName, v6PoolName := createIaaSPoolPerFamily(markGlobalIaaSPool)

		By("write global metadata binding every pool IP to another node: no local cache hit is possible")
		writePoolMetadata(v4PoolName, "", master, globalEntriesForPool(v4PoolName, "ghost-node-e2e"))
		writePoolMetadata(v6PoolName, "", master, globalEntriesForPool(v6PoolName, "ghost-node-e2e"))

		smcName := setupProviderNetwork(namespace, v4PoolName, v6PoolName, master)

		By("create the Pod: stealing the idle entry from the other node requires a provider RPC")
		pod := startProviderPod(namespace, smcName, node)
		expectProviderCall(providerMockAllocatePath, pod.Name, namespace)

		deleteProviderPod(pod)
	})

	It("allocates a strict IPv4/IPv6 pair from a node-level paired pool set without any provider RPC", Label("I00029"), func() {
		v6PoolName, v6Pool := common.GenerateExampleIpv6poolObject(5)
		markIaaSProviderPool(v6Pool)
		v6Pool.Spec.NodeName = []string{node.Name}
		createPoolWithCleanup(v6Pool, v6PoolName)

		v4PoolName, v4Pool := common.GenerateExampleIpv4poolObject(5)
		markIaaSProviderPool(v4Pool)
		v4Pool.Annotations[constant.AnnoIPPoolPairPool] = v6PoolName
		v4Pool.Spec.NodeName = []string{node.Name}
		createPoolWithCleanup(v4Pool, v4PoolName)

		By("write out-of-order v4->v6 pair entries on the primary pool only")
		v4IPs := sortedPoolIPs(v4PoolName)
		v6IPs := sortedPoolIPs(v6PoolName)
		entries := make(map[string]spiderpoolv2beta1.IPMetadataEntry, len(v4IPs))
		for i, v4 := range v4IPs {
			// Rotate the v6 side so the pairing is NOT ordinal: the lowest
			// v4 must come with the entry's own ipv6, not the lowest v6.
			pairedV6 := v6IPs[(i+2)%len(v6IPs)].String()
			entries[v4.String()] = spiderpoolv2beta1.IPMetadataEntry{
				IPv6: ptr.To(pairedV6),
				MAC:  prewarmEntryMAC,
				VLAN: ptr.To(prewarmEntryVLAN),
			}
		}
		writePoolMetadata(v4PoolName, node.Name, master, entries)
		expectedV4 := v4IPs[0].String()
		expectedV6 := v6IPs[2].String()

		smcName := setupProviderNetwork(namespace, v4PoolName, v6PoolName, master)

		pod := startProviderPod(namespace, smcName, node)

		By("verify the pair allocation was a zero-RPC prewarm cache hit")
		expectNoProviderCall(providerMockAllocatePath, pod.Name, namespace)

		By("verify the strict pair mapping from the metadata entry, not ordinal ordering")
		detail := endpointFirstIPDetail(pod)
		Expect(detail.IPv4).NotTo(BeNil())
		Expect(normalizeIPAddress(*detail.IPv4)).To(Equal(expectedV4))
		Expect(detail.IPv6).NotTo(BeNil())
		Expect(normalizeIPAddress(*detail.IPv6)).To(Equal(expectedV6), "the IPv6 must be the metadata-paired address of the chosen IPv4")
		Expect(detail.MAC).NotTo(BeNil())
		Expect(*detail.MAC).To(Equal(prewarmEntryMAC))

		deleteProviderPod(pod)
		By("verify the release kept the cloud-side reservation: zero provider release RPC")
		expectNoProviderCall(providerMockReleasePath, pod.Name, namespace)
	})

	It("serves a global paired pool cold path atomically for both families via one provider RPC", Label("I00030"), func() {
		v6PoolName, v6Pool := common.GenerateExampleIpv6poolObject(5)
		markGlobalIaaSPool(v6Pool)
		createPoolWithCleanup(v6Pool, v6PoolName)

		v4PoolName, v4Pool := common.GenerateExampleIpv4poolObject(5)
		markGlobalIaaSPool(v4Pool)
		v4Pool.Annotations[constant.AnnoIPPoolPairPool] = v6PoolName
		createPoolWithCleanup(v4Pool, v4PoolName)

		By("write the global metadata skeleton on both pools (parentNic only, no entries)")
		writePoolMetadata(v4PoolName, "", master, nil)
		writePoolMetadata(v6PoolName, "", master, nil)

		smcName := setupProviderNetwork(namespace, v4PoolName, v6PoolName, master)

		By("create the Pod: the empty pair entry set must cold-path both families through one provider RPC")
		pod := startProviderPod(namespace, smcName, node)
		expectProviderCall(providerMockAllocatePath, pod.Name, namespace)

		By("verify both families were allocated and match the provider mock IP cache")
		expectSpiderEndpointMatchesProviderCache(pod)

		deleteProviderPod(pod)
		By("verify the release kept the cloud-side reservation: zero provider release RPC")
		expectNoProviderCall(providerMockReleasePath, pod.Name, namespace)
	})
})

// createIaaSPoolPerFamily creates one IPv4 and one IPv6 IPPool, applies the
// given marker mutation to both, and registers their cleanup.
func createIaaSPoolPerFamily(mutate func(pool *spiderpoolv2beta1.SpiderIPPool)) (v4PoolName, v6PoolName string) {
	v4PoolName, v4Pool := common.GenerateExampleIpv4poolObject(5)
	mutate(v4Pool)
	createPoolWithCleanup(v4Pool, v4PoolName)

	v6PoolName, v6Pool := common.GenerateExampleIpv6poolObject(5)
	mutate(v6Pool)
	createPoolWithCleanup(v6Pool, v6PoolName)
	return v4PoolName, v6PoolName
}

func createPoolWithCleanup(pool *spiderpoolv2beta1.SpiderIPPool, poolName string) {
	By("create the IPPool " + poolName)
	Expect(common.CreateIppool(frame, pool)).To(Succeed())
	DeferCleanup(func() {
		if CurrentSpecReport().Failed() {
			return
		}
		By("delete the IPPool " + poolName)
		Expect(common.DeleteIPPoolByName(frame, poolName)).To(Succeed())
	})
}

// markGlobalIaaSPool sets the iaas-global annotation on the pool; the IPPool
// mutating webhook syncs the matching label, which is the marker consumed by
// the global-pool allocation logic.
func markGlobalIaaSPool(pool *spiderpoolv2beta1.SpiderIPPool) {
	if pool.Annotations == nil {
		pool.Annotations = map[string]string{}
	}
	pool.Annotations[constant.AnnoIPPoolIaasGlobal] = "true"
}

// writePoolMetadata acts as the external IaaS provider controller: it writes
// status.ipMetaData with the authoritative metadata JSON
// ({"scope": ..., "parentNic": ..., "ips": {...}}) and stamps
// observedGeneration with the pool's current generation so the snapshot is
// considered ready by the agent.
func writePoolMetadata(poolName, scope, parentNic string, entries map[string]spiderpoolv2beta1.IPMetadataEntry) {
	payload := map[string]interface{}{
		"scope": scope,
	}
	if parentNic != "" {
		payload[constant.IPPoolMetadataParentNicKey] = parentNic
	}
	if len(entries) > 0 {
		payload["ips"] = entries
	}
	raw, err := json.Marshal(payload)
	Expect(err).NotTo(HaveOccurred())
	metadata := string(raw)

	Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		pool, err := common.GetIppoolByName(frame, poolName)
		if err != nil {
			return err
		}
		pool.Status.IPMetaData = &spiderpoolv2beta1.IPMetaData{
			Metadata:           &metadata,
			ObservedGeneration: ptr.To(pool.Generation),
			ReadyIPCount:       ptr.To(int64(len(entries))),
		}
		return frame.KClient.Status().Update(context.Background(), pool)
	})).To(Succeed(), "failed to write provider metadata to IPPool %s", poolName)
	GinkgoWriter.Printf("wrote provider metadata to IPPool %s: %s\n", poolName, metadata)
}

// nodeLevelEntriesForPool builds one node-scoped metadata entry (MAC+VLAN,
// no per-entry node) for every IP of the pool.
func nodeLevelEntriesForPool(poolName string) map[string]spiderpoolv2beta1.IPMetadataEntry {
	entries := make(map[string]spiderpoolv2beta1.IPMetadataEntry)
	for _, ip := range sortedPoolIPs(poolName) {
		entries[ip.String()] = spiderpoolv2beta1.IPMetadataEntry{
			MAC:  prewarmEntryMAC,
			VLAN: ptr.To(prewarmEntryVLAN),
		}
	}
	return entries
}

// globalEntriesForPool builds one global-pool metadata entry bound to the
// given node for every IP of the pool.
func globalEntriesForPool(poolName, boundNode string) map[string]spiderpoolv2beta1.IPMetadataEntry {
	entries := make(map[string]spiderpoolv2beta1.IPMetadataEntry)
	for _, ip := range sortedPoolIPs(poolName) {
		entries[ip.String()] = spiderpoolv2beta1.IPMetadataEntry{
			MAC:  prewarmEntryMAC,
			VLAN: ptr.To(prewarmEntryVLAN),
			Node: ptr.To(boundNode),
		}
	}
	return entries
}

func sortedPoolIPs(poolName string) []net.IP {
	pool, err := common.GetIppoolByName(frame, poolName)
	Expect(err).NotTo(HaveOccurred())
	Expect(pool.Spec.IPVersion).NotTo(BeNil())
	ips, err := spiderpoolip.ParseIPRanges(*pool.Spec.IPVersion, pool.Spec.IPs)
	Expect(err).NotTo(HaveOccurred())
	Expect(ips).NotTo(BeEmpty())
	sort.Slice(ips, func(i, j int) bool {
		return string(ips[i].To16()) < string(ips[j].To16())
	})
	return ips
}

// setupProviderNetwork creates a VLAN SpiderMultusConfig referencing the two
// pools, waits for its NetworkAttachmentDefinition, and registers cleanup.
func setupProviderNetwork(namespace, v4PoolName, v6PoolName, master string) string {
	smcName := "vlan-prewarm-" + common.GenerateString(10, true)
	smc := newVlanSpiderMultusConfigWithMaster(namespace, smcName, v4PoolName, v6PoolName, master)
	By("create a VLAN SpiderMultusConfig " + smcName + " referencing the IPPools")
	Expect(frame.CreateSpiderMultusInstance(smc)).To(Succeed())
	DeferCleanup(func() {
		if CurrentSpecReport().Failed() {
			return
		}
		By("delete the VLAN SpiderMultusConfig " + smcName)
		Expect(frame.DeleteSpiderMultusInstance(namespace, smcName)).To(Succeed())
	})
	By("wait for the NetworkAttachmentDefinition " + smcName + " to become ready")
	waitNetworkAttachmentReady(smcName, namespace)
	return smcName
}

func startProviderPod(namespace, smcName string, node *corev1.Node) *corev1.Pod {
	podName := "provider-pod-" + common.GenerateString(8, true)
	pod := newProviderPod(podName, namespace, smcName, node)
	By("create the provider Pod " + namespace + "/" + podName + " on node " + node.Name)
	Expect(frame.CreatePod(pod)).To(Succeed())

	ctx, cancel := context.WithTimeout(context.Background(), common.PodStartTimeout)
	defer cancel()
	runningPod, err := frame.WaitPodStarted(podName, namespace, ctx)
	Expect(err).NotTo(HaveOccurred())
	return runningPod
}

func deleteProviderPod(pod *corev1.Pod) {
	By("delete the provider Pod " + pod.Namespace + "/" + pod.Name)
	ctx, cancel := context.WithTimeout(context.Background(), common.ResourceDeleteTimeout)
	defer cancel()
	Expect(frame.DeletePodUntilFinish(pod.Name, pod.Namespace, ctx)).To(Succeed())
}

// expectNoProviderCall asserts that the mock provider consistently has NO
// record of the given RPC for the Pod: zero-RPC paths (prewarm cache hits
// and IaaS-pool releases) must never reach the provider.
func expectNoProviderCall(path, podName, namespace string) {
	Consistently(func(g Gomega) {
		records, err := providerMock.Records()
		g.Expect(err).NotTo(HaveOccurred())
		record := records.find(path, podName, namespace)
		if record != nil {
			GinkgoWriter.Printf("unexpected provider mock record for %s Pod %s/%s: %+v\n", path, namespace, podName, record)
		}
		g.Expect(record).To(BeNil())
	}).WithTimeout(10 * time.Second).WithPolling(2 * time.Second).Should(Succeed())
}

func endpointFirstIPDetail(pod *corev1.Pod) spiderpoolv2beta1.IPAllocationDetail {
	var detail spiderpoolv2beta1.IPAllocationDetail
	Eventually(func(g Gomega) {
		endpoint, err := common.GetWorkloadByName(frame, pod.Namespace, pod.Name)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(endpoint.Status.Current.IPs).NotTo(BeEmpty())
		detail = endpoint.Status.Current.IPs[0]
	}).WithTimeout(common.EventOccurTimeout).WithPolling(time.Second).Should(Succeed())
	return detail
}
