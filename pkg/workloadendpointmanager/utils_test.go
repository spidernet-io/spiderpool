// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"fmt"

	"github.com/moby/moby/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var _ = Describe("WorkloadEndpointManager utils", Label("workloadendpoint_manager_utils_test"), func() {
	var endpointT *spiderpoolv1.SpiderEndpoint

	BeforeEach(func() {
		endpointT = &spiderpoolv1.SpiderEndpoint{
			TypeMeta: metav1.TypeMeta{
				Kind:       constant.SpiderEndpointKind,
				APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "endpoint",
				Namespace: "default",
			},
			Status: spiderpoolv1.WorkloadEndpointStatus{},
		}
	})

	Describe("Test RetrieveIPAllocation", func() {
		var nic1, nic2 string
		var containerID, historyContainerID string
		var allocationT, historyAllocationT *spiderpoolv1.PodIPAllocation

		BeforeEach(func() {
			nic1 = "eth0"
			nic2 = "net1"

			containerID = stringid.GenerateRandomID()
			allocationT = &spiderpoolv1.PodIPAllocation{
				ContainerID: containerID,
				IPs: []spiderpoolv1.IPAllocationDetail{
					{
						NIC:      nic1,
						Vlan:     pointer.Int64(0),
						IPv4:     pointer.String("172.18.40.10/24"),
						IPv4Pool: pointer.String("ipv4-ippool-1"),
					},
					{
						NIC:      nic2,
						Vlan:     pointer.Int64(0),
						IPv4:     pointer.String("192.168.40.9/24"),
						IPv4Pool: pointer.String("ipv4-ippool-2"),
					},
				},
			}

			historyContainerID = stringid.GenerateRandomID()
			historyAllocationT = &spiderpoolv1.PodIPAllocation{
				ContainerID: historyContainerID,
				IPs: []spiderpoolv1.IPAllocationDetail{
					{
						NIC:      nic1,
						Vlan:     pointer.Int64(0),
						IPv4:     pointer.String("172.18.40.5/24"),
						IPv4Pool: pointer.String("ipv4-ippool-1"),
					},
					{
						NIC:      nic2,
						Vlan:     pointer.Int64(0),
						IPv4:     pointer.String("192.168.40.6/24"),
						IPv4Pool: pointer.String("ipv4-ippool-2"),
					},
				},
			}
		})

		It("inputs nil Endpoint", func() {
			allocation := workloadendpointmanager.RetrieveCIDIPAllocation(containerID, nic2, nil)
			Expect(allocation).To(BeNil())
		})

		It("retrieves the IP allocation but the current record is nil", func() {
			allocation := workloadendpointmanager.RetrieveCIDIPAllocation(containerID, nic2, endpointT)
			Expect(allocation).To(BeNil())
		})

		It("retrieves non-existent containerID IP allocation", func() {
			endpointT.Status.Current = allocationT
			endpointT.Status.History = append(endpointT.Status.History, *allocationT, *historyAllocationT)

			allocation := workloadendpointmanager.RetrieveCIDIPAllocation(stringid.GenerateRandomID(), nic2, endpointT)
			Expect(allocation).To(BeNil())
		})

		It("retrieves the IP allocation with the specified containerID", func() {
			endpointT.Status.Current = allocationT
			endpointT.Status.History = append(endpointT.Status.History, *allocationT, *historyAllocationT)

			allocation := workloadendpointmanager.RetrieveCIDIPAllocation(containerID, nic2, endpointT)
			Expect(allocation).To(Equal(allocationT))

			historyAllocation := workloadendpointmanager.RetrieveCIDIPAllocation(historyContainerID, nic2, endpointT)
			Expect(historyAllocation).To(Equal(historyAllocationT))
		})
	})

	PDescribe("Test ListAllHistoricalIPs", func() {})
})
