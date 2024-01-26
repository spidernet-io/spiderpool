// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var _ = Describe("WorkloadEndpointManager utils", Label("workloadendpoint_manager_utils_test"), func() {
	var endpointT *spiderpoolv2beta1.SpiderEndpoint

	BeforeEach(func() {
		endpointT = &spiderpoolv2beta1.SpiderEndpoint{
			TypeMeta: metav1.TypeMeta{
				Kind:       constant.KindSpiderEndpoint,
				APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "endpoint",
				Namespace: "default",
			},
			Status: spiderpoolv2beta1.WorkloadEndpointStatus{},
		}
	})

	Describe("Test RetrieveIPAllocation", func() {
		var nic1, nic2 string
		var uid string
		var allocationT spiderpoolv2beta1.PodIPAllocation

		BeforeEach(func() {
			nic1 = "eth0"
			nic2 = "net1"

			uid = string(uuid.NewUUID())
			allocationT = spiderpoolv2beta1.PodIPAllocation{
				UID: uid,
				IPs: []spiderpoolv2beta1.IPAllocationDetail{
					{
						NIC:      nic1,
						Vlan:     ptr.To(int64(0)),
						IPv4:     ptr.To("172.18.40.10/24"),
						IPv4Pool: ptr.To("ipv4-ippool-1"),
					},
					{
						NIC:      nic2,
						Vlan:     ptr.To(int64(0)),
						IPv4:     ptr.To("192.168.40.9/24"),
						IPv4Pool: ptr.To("ipv4-ippool-2"),
					},
				},
			}
		})

		It("inputs nil Endpoint", func() {
			allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic2, nil, false)
			Expect(allocation).To(BeNil())
		})

		It("retrieves non-existent current IP allocation", func() {
			endpointT.Status.Current = allocationT

			allocation := workloadendpointmanager.RetrieveIPAllocation(string(uuid.NewUUID()), nic2, endpointT, false)
			Expect(allocation).To(BeNil())
		})

		It("retrieves the current IP allocation", func() {
			endpointT.Status.Current = allocationT

			allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic2, endpointT, false)
			Expect(allocation).NotTo(BeNil())
			Expect(*allocation).To(Equal(allocationT))
		})

		It("retrieves the IP allocation of Static IP", func() {
			endpointT.Status.Current = allocationT

			allocation := workloadendpointmanager.RetrieveIPAllocation(string(uuid.NewUUID()), nic2, endpointT, true)
			Expect(allocation).NotTo(BeNil())
			Expect(*allocation).To(Equal(allocationT))
		})

		It("retrieves the IP allocation of empty nic", func() {
			allocationT.IPs = append(allocationT.IPs, spiderpoolv2beta1.IPAllocationDetail{
				NIC:      "",
				Vlan:     ptr.To(int64(0)),
				IPv4:     ptr.To("10.60.1.2/24"),
				IPv4Pool: ptr.To("ipv4-ippool-3"),
			})

			endpointT.Status.Current = allocationT
			allocation := workloadendpointmanager.RetrieveIPAllocation(uid, "net2", endpointT, false)
			Expect(allocation).NotTo(BeNil())
			Expect(*allocation).To(Equal(allocationT))
		})
	})
})
