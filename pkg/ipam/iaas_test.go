// Copyright 2026 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ipam

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/spidernet-io/spiderpool/api/v1/agent/models"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	iaasclient "github.com/spidernet-io/spiderpool/pkg/iaas/client"
	v2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
)

type fakeIaaSClient struct {
	allocateRequests []*iaasclient.AllocateIPRequest
	cache            map[string]string
}

func (f *fakeIaaSClient) AllocateIPs(_ context.Context, req *iaasclient.AllocateIPRequest) (*iaasclient.AllocateIPResponse, error) {
	f.allocateRequests = append(f.allocateRequests, req)
	response := make([]iaasclient.SubEniResult, 0, len(req.SubEniRequests))
	for _, item := range req.SubEniRequests {
		response = append(response, iaasclient.SubEniResult{
			ParentNicMac: item.ParentNicMac,
			Subnet:       item.Subnet,
			IPv4Address:  item.IPv4Address,
			IPv6Address:  item.IPv6Address,
			MacAddress:   "02:00:00:00:00:01",
			VlanID:       100,
		})
	}
	return &iaasclient.AllocateIPResponse{SubEniResponses: response}, nil
}

func (f *fakeIaaSClient) ReleaseIP(context.Context, *iaasclient.ReleaseIPRequest) error {
	return nil
}

func (f *fakeIaaSClient) GetCachedParentNicMac(subnet string) (string, bool) {
	value, ok := f.cache[subnet]
	return value, ok
}

func (f *fakeIaaSClient) CacheParentNicMac(subnet, mac string) {
	if f.cache == nil {
		f.cache = map[string]string{}
	}
	f.cache[subnet] = mac
}

var _ = Describe("IaaS provider pool filtering", Label("ipam_iaas_test"), func() {
	newIaaSPool := func(name string) *v2beta1.SpiderIPPool {
		return &v2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{constant.LabelIPPoolIaasProvider: "huaweicloud"},
			},
		}
	}
	newGlobalIaaSPool := func(name string) *v2beta1.SpiderIPPool {
		return &v2beta1.SpiderIPPool{
			ObjectMeta: metav1.ObjectMeta{
				Name:   name,
				Labels: map[string]string{constant.LabelIPPoolIaasProvider: "huaweicloud"},
			},
		}
	}
	newPlainPool := func(name string) *v2beta1.SpiderIPPool {
		return &v2beta1.SpiderIPPool{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}

	It("submits only IaaS-pool results from a mixed-pool Pod", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newPlainPool("plain-pool"),
			newIaaSPool("pool-v4"),
			newIaaSPool("pool-v6"),
		).Build()
		// Warm subnet-keyed cache stands in for the SMC/netlink resolution.
		client := &fakeIaaSClient{cache: map[string]string{
			"10.0.1.0/24":      "02:00:00:00:00:02",
			"fd00:10:0:1::/64": "02:00:00:00:00:02",
		}}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-a",
				Namespace: "tenant-a",
				UID:       "pod-uid",
			},
			Spec: corev1.PodSpec{NodeName: "node-a"},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "plain-pool"}},
			{IP: &models.IPConfig{Address: ptr.To("10.0.1.2/24"), Nic: ptr.To("net1"), Version: ptr.To[int64](4), IPPool: "pool-v4"}},
			{IP: &models.IPConfig{Address: ptr.To("fd00:10:0:1::2/64"), Nic: ptr.To("net1"), Version: ptr.To[int64](6), IPPool: "pool-v6"}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.allocateRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].SubEniRequests).To(ConsistOf(
			iaasclient.SubEniRequest{
				ParentNicMac: "02:00:00:00:00:02",
				Subnet:       "10.0.1.0/24",
				IPv4Address:  "10.0.1.2",
				IPv6Address:  "fd00:10:0:1::2",
				IPv4PoolName: "pool-v4",
				IPv6PoolName: "pool-v6",
			},
		))
		Expect(results[0].IP.Mac).To(BeEmpty())
		Expect(results[0].IP.Vlan).To(BeZero())
		Expect(results[1].IP.Mac).To(Equal("02:00:00:00:00:01"))
		Expect(results[1].IP.Vlan).To(Equal(int64(100)))
		Expect(results[2].IP.Mac).To(Equal("02:00:00:00:00:01"))
		Expect(results[2].IP.Vlan).To(Equal(int64(100)))
	})

	It("passes through a single-stack allocation without enforcing a dual-stack pair", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newGlobalIaaSPool("gpool-v4"),
		).Build()
		client := &fakeIaaSClient{cache: map[string]string{
			"10.0.1.0/24": "02:00:00:00:00:02",
		}}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-a",
				Namespace: "tenant-a",
				UID:       "pod-uid",
			},
			Spec: corev1.PodSpec{NodeName: "node-a"},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.1.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "gpool-v4"}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.allocateRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].SubEniRequests).To(ConsistOf(
			iaasclient.SubEniRequest{
				ParentNicMac: "02:00:00:00:00:02",
				Subnet:       "10.0.1.0/24",
				IPv4Address:  "10.0.1.2",
				IPv6Address:  "",
				IPv4PoolName: "gpool-v4",
			},
		))
		Expect(results[0].IP.Mac).To(Equal("02:00:00:00:00:01"))
		Expect(results[0].IP.Vlan).To(Equal(int64(100)))
	})

	It("passes through an IPv6-only allocation as-is", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newIaaSPool("pool-v6"),
		).Build()
		client := &fakeIaaSClient{cache: map[string]string{
			"fd00:10:0:1::/64": "02:00:00:00:00:02",
		}}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-a",
				Namespace: "tenant-a",
				UID:       "pod-uid",
			},
			Spec: corev1.PodSpec{NodeName: "node-a"},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("fd00:10:0:1::2/64"), Nic: ptr.To("eth0"), Version: ptr.To[int64](6), IPPool: "pool-v6"}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.allocateRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].SubEniRequests).To(ConsistOf(
			iaasclient.SubEniRequest{
				ParentNicMac: "02:00:00:00:00:02",
				Subnet:       "fd00:10:0:1::/64",
				IPv4Address:  "",
				IPv6Address:  "fd00:10:0:1::2",
				IPv6PoolName: "pool-v6",
			},
		))
		Expect(results[0].IP.Mac).To(Equal("02:00:00:00:00:01"))
		Expect(results[0].IP.Vlan).To(Equal(int64(100)))
	})

	It("does not call IaaS for a Pod whose pool is not IaaS-managed", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newPlainPool("plain-pool"),
		).Build()
		client := &fakeIaaSClient{}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "tenant-a"},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "plain-pool"}},
		}

		response, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(response).To(BeNil())
		Expect(client.allocateRequests).To(BeEmpty())
	})

	It("allocates via the subnet-keyed parentNicMac cache", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newIaaSPool("pool-v4"),
		).Build()
		// Warm cache keyed by subnet stands in for the full resolution
		// chain (Multus annotation -> SpiderMultusConfig -> netlink).
		client := &fakeIaaSClient{cache: map[string]string{
			"10.0.0.0/24": "aa:bb:cc:dd:ee:ff",
		}}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "tenant-a"},
			Spec:       corev1.PodSpec{NodeName: "node-a"},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "pool-v4"}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.allocateRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].SubEniRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].SubEniRequests[0].ParentNicMac).To(Equal("aa:bb:cc:dd:ee:ff"))
		Expect(client.allocateRequests[0].SubEniRequests[0].Subnet).To(Equal("10.0.0.0/24"))
	})

	It("allocates via the node-level pool's agent-published status.parentNic MAC", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		nodePool := newIaaSPool("pool-v4")
		nodePool.Spec.NodeName = []string{"node-a"}
		nodePool.Status.ParentNic = &v2beta1.ParentNicStatus{Name: "eth1", MAC: "fa:16:3e:aa:bb:cc"}
		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(nodePool).Build()
		// Cold cache: the MAC must come from status.parentNic, with no
		// Multus/SMC resolution involved.
		client := &fakeIaaSClient{}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "tenant-a"},
			Spec:       corev1.PodSpec{NodeName: "node-a"},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "pool-v4"}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.allocateRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].SubEniRequests[0].ParentNicMac).To(Equal("fa:16:3e:aa:bb:cc"))
		// The resolved MAC is cached by subnet for the release path.
		Expect(client.cache).To(HaveKeyWithValue("10.0.0.0/24", "fa:16:3e:aa:bb:cc"))
	})

	It("fails closed when the parent NIC MAC cannot be resolved", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			newIaaSPool("pool-v4"),
		).Build()
		// Cold cache and a Pod without Multus annotations: the resolution
		// chain falls through to the SMC step and fails there, so the
		// allocation must fail closed.
		client := &fakeIaaSClient{}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "tenant-a"},
			Spec:       corev1.PodSpec{NodeName: "node-a"},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "pool-v4"}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("parent NIC"))
		Expect(client.allocateRequests).To(BeEmpty())
	})
})

var _ = Describe("IaaS parent NIC resolution from SpiderMultusConfig", Label("ipam_iaas_test"), func() {
	newSMC := func(cniType string) *v2beta1.SpiderMultusConfig {
		return &v2beta1.SpiderMultusConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "iaas-net", Namespace: "spiderpool"},
			Spec:       v2beta1.MultusCNIConfigSpec{CniType: ptr.To(cniType)},
		}
	}

	It("resolves the master from an eni-vlan SpiderMultusConfig", func() {
		smc := newSMC(constant.EniVlanCNI)
		smc.Spec.EniVlanConfig = &v2beta1.SpiderEniVlanCniConfig{Master: []string{"eth1"}}

		master, err := getMasterIfaceFromMultusConfig(smc)
		Expect(err).NotTo(HaveOccurred())
		Expect(master).To(Equal("eth1"))
	})

	It("resolves the master from macvlan and ipvlan SpiderMultusConfigs", func() {
		smc := newSMC(constant.MacvlanCNI)
		smc.Spec.MacvlanConfig = &v2beta1.SpiderMacvlanCniConfig{Master: []string{"eth1"}}
		master, err := getMasterIfaceFromMultusConfig(smc)
		Expect(err).NotTo(HaveOccurred())
		Expect(master).To(Equal("eth1"))

		smc = newSMC(constant.IPVlanCNI)
		smc.Spec.IPVlanConfig = &v2beta1.SpiderIPvlanCniConfig{Master: []string{"eth2"}}
		master, err = getMasterIfaceFromMultusConfig(smc)
		Expect(err).NotTo(HaveOccurred())
		Expect(master).To(Equal("eth2"))
	})

	It("fails closed for the community static vlan CNI with an IaaS pool", func() {
		smc := newSMC(constant.VlanCNI)
		smc.Spec.VlanConfig = &v2beta1.SpiderVlanCniConfig{
			Master: []string{"eth1"},
			VlanID: ptr.To(int32(100)),
		}

		_, err := getMasterIfaceFromMultusConfig(smc)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported CniType vlan"))
	})
})
