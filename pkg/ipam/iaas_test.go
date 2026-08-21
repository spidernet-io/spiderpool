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

func (f *fakeIaaSClient) GetCachedParentNicMac(key string) (string, bool) {
	value, ok := f.cache[key]
	return value, ok
}

func (f *fakeIaaSClient) CacheParentNicMac(key, mac string) {
	f.cache[key] = mac
}

var _ = Describe("IaaS provider network filtering", Label("ipam_iaas_test"), func() {
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
				Labels: map[string]string{constant.LabelIPPoolIaasGlobal: "true"},
			},
		}
	}
	newPlainPool := func(name string) *v2beta1.SpiderIPPool {
		return &v2beta1.SpiderIPPool{ObjectMeta: metav1.ObjectMeta{Name: name}}
	}

	It("recognizes master-carrying SpiderMultusConfigs (vlan, macvlan, ipvlan)", func() {
		macvlanType := constant.MacvlanCNI
		sriovType := constant.SriovCNI
		vlanType := constant.VlanCNI
		vlanID := int32(100)

		Expect(hasMasterInterface(nil)).To(BeFalse())
		Expect(hasMasterInterface(&v2beta1.SpiderMultusConfig{
			Spec: v2beta1.MultusCNIConfigSpec{CniType: &sriovType},
		})).To(BeFalse())
		Expect(hasMasterInterface(&v2beta1.SpiderMultusConfig{
			Spec: v2beta1.MultusCNIConfigSpec{CniType: &vlanType},
		})).To(BeFalse())
		Expect(hasMasterInterface(&v2beta1.SpiderMultusConfig{
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType:       &macvlanType,
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{Master: []string{"eth1"}},
			},
		})).To(BeTrue())
		Expect(hasMasterInterface(&v2beta1.SpiderMultusConfig{
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType:    &vlanType,
				VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeManual), VlanID: &vlanID},
			},
		})).To(BeTrue())
		Expect(hasMasterInterface(&v2beta1.SpiderMultusConfig{
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType:    &vlanType,
				VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeAuto)},
			},
		})).To(BeTrue())
	})

	It("submits only IaaS-pool results from a mixed-network Pod", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		macvlanType := constant.MacvlanCNI
		vlanType := constant.VlanCNI
		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "macvlan-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:       &macvlanType,
					MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{Master: []string{"eth1"}},
				},
			},
			&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "provider-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:    &vlanType,
					VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeAuto), Master: []string{"eth2"}},
				},
			},
			newPlainPool("plain-pool"),
			newIaaSPool("pool-v4"),
			newIaaSPool("pool-v6"),
		).Build()
		client := &fakeIaaSClient{
			cache: map[string]string{"tenant-a/provider-net": "02:00:00:00:00:02"},
		}
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
				Annotations: map[string]string{
					constant.MultusDefaultNetAnnot:        "tenant-a/macvlan-net",
					constant.MultusNetworkAttachmentAnnot: "tenant-a/provider-net",
				},
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

		vlanType := constant.VlanCNI
		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "provider-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:    &vlanType,
					VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeAuto), Master: []string{"eth2"}},
				},
			},
			newGlobalIaaSPool("gpool-v4"),
		).Build()
		client := &fakeIaaSClient{
			cache: map[string]string{"tenant-a/provider-net": "02:00:00:00:00:02"},
		}
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
				Annotations: map[string]string{
					constant.MultusDefaultNetAnnot: "tenant-a/provider-net",
				},
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

		vlanType := constant.VlanCNI
		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "provider-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:    &vlanType,
					VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanMode: ptr.To(constant.VlanModeAuto), Master: []string{"eth2"}},
				},
			},
			newIaaSPool("pool-v6"),
		).Build()
		client := &fakeIaaSClient{
			cache: map[string]string{"tenant-a/provider-net": "02:00:00:00:00:02"},
		}
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
				Annotations: map[string]string{
					constant.MultusDefaultNetAnnot: "tenant-a/provider-net",
				},
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

		macvlanType := constant.MacvlanCNI
		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "macvlan-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:       &macvlanType,
					MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{Master: []string{"eth1"}},
				},
			},
			newPlainPool("plain-pool"),
		).Build()
		client := &fakeIaaSClient{cache: map[string]string{}}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "tenant-a",
				Annotations: map[string]string{
					constant.MultusDefaultNetAnnot: "tenant-a/macvlan-net",
				},
			},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "plain-pool"}},
		}

		response, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(response).To(BeNil())
		Expect(client.allocateRequests).To(BeEmpty())
	})

	It("allocates via the provider for an IaaS pool over a macvlan network", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		macvlanType := constant.MacvlanCNI
		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "macvlan-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:       &macvlanType,
					MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{Master: []string{"eth1"}},
				},
			},
			newIaaSPool("pool-v4"),
		).Build()
		client := &fakeIaaSClient{cache: map[string]string{
			// Warm cache stands in for the netlink lookup of eth1.
			"tenant-a/macvlan-net": "aa:bb:cc:dd:ee:ff",
		}}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "tenant-a",
				Annotations: map[string]string{
					constant.MultusDefaultNetAnnot: "tenant-a/macvlan-net",
				},
			},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "pool-v4"}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.allocateRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].SubEniRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].SubEniRequests[0].ParentNicMac).To(Equal("aa:bb:cc:dd:ee:ff"))
	})

	It("fails closed when an IaaS pool is used over a network without a master interface", func() {
		scheme := runtime.NewScheme()
		Expect(v2beta1.AddToScheme(scheme)).To(Succeed())

		sriovType := constant.SriovCNI
		apiReader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
			&v2beta1.SpiderMultusConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "sriov-net", Namespace: "tenant-a"},
				Spec: v2beta1.MultusCNIConfigSpec{
					CniType:     &sriovType,
					SriovConfig: &v2beta1.SpiderSRIOVCniConfig{},
				},
			},
			newIaaSPool("pool-v4"),
		).Build()
		client := &fakeIaaSClient{cache: map[string]string{}}
		instance := &ipam{config: IPAMConfig{
			AgentNamespace: "kube-system",
			APIReader:      apiReader,
			IaaSClient:     client,
		}}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "tenant-a",
				Annotations: map[string]string{
					constant.MultusDefaultNetAnnot: "tenant-a/sriov-net",
				},
			},
		}
		results := []*spiderpooltypes.AllocationResult{
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4), IPPool: "pool-v4"}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("unsupported CniType"))
		Expect(client.allocateRequests).To(BeEmpty())
	})
})
