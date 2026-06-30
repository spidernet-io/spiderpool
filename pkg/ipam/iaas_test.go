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
	response := make([]iaasclient.IaaSIPAllocationResult, 0, len(req.IaaSIPsAllocationRequest))
	for _, item := range req.IaaSIPsAllocationRequest {
		response = append(response, iaasclient.IaaSIPAllocationResult{
			IPAddress:  item.IPAddress,
			MacAddress: "02:00:00:00:00:01",
			VlanID:     100,
		})
	}
	return &iaasclient.AllocateIPResponse{IaaSIPsAllocationResponse: response}, nil
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
	It("recognizes only provider-managed VLAN SpiderMultusConfigs", func() {
		macvlanType := constant.MacvlanCNI
		vlanType := constant.VlanCNI
		vlanID := int32(100)

		Expect(isProviderVLANSpiderMultusConfig(nil)).To(BeFalse())
		Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType:       &macvlanType,
				MacvlanConfig: &v2beta1.SpiderMacvlanCniConfig{},
			},
		})).To(BeFalse())
		Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType:    &vlanType,
				VlanConfig: &v2beta1.SpiderVlanCniConfig{VlanID: &vlanID},
			},
		})).To(BeFalse())
		Expect(isProviderVLANSpiderMultusConfig(&v2beta1.SpiderMultusConfig{
			Spec: v2beta1.MultusCNIConfigSpec{
				CniType:    &vlanType,
				VlanConfig: &v2beta1.SpiderVlanCniConfig{},
			},
		})).To(BeTrue())
	})

	It("submits only provider VLAN results from a mixed-network Pod", func() {
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
					VlanConfig: &v2beta1.SpiderVlanCniConfig{Master: []string{"eth2"}},
				},
			},
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
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4)}},
			{IP: &models.IPConfig{Address: ptr.To("10.0.1.2/24"), Nic: ptr.To("net1"), Version: ptr.To[int64](4)}},
		}

		_, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(client.allocateRequests).To(HaveLen(1))
		Expect(client.allocateRequests[0].IaaSIPsAllocationRequest).To(ConsistOf(
			iaasclient.IaaSIPAllocationItem{
				IPAddress:    "10.0.1.2",
				Subnet:       "10.0.1.0/24",
				ParentNicMac: "02:00:00:00:00:02",
			},
		))
		Expect(results[0].IP.Mac).To(BeEmpty())
		Expect(results[0].IP.Vlan).To(BeZero())
		Expect(results[1].IP.Mac).To(Equal("02:00:00:00:00:01"))
		Expect(results[1].IP.Vlan).To(Equal(int64(100)))
	})

	It("does not call IaaS for a Pod that references only macvlan", func() {
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
			{IP: &models.IPConfig{Address: ptr.To("10.0.0.2/24"), Nic: ptr.To("eth0"), Version: ptr.To[int64](4)}},
		}

		response, err := instance.callIaaSAllocate(context.Background(), pod, results)
		Expect(err).NotTo(HaveOccurred())
		Expect(response).To(BeNil())
		Expect(client.allocateRequests).To(BeEmpty())
	})
})
