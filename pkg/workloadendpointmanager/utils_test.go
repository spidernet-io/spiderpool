// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
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
				Kind:       constant.KindSpiderEndpoint,
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
		var uid string
		var allocationT *spiderpoolv1.PodIPAllocation

		BeforeEach(func() {
			nic1 = "eth0"
			nic2 = "net1"

			uid = string(uuid.NewUUID())
			allocationT = &spiderpoolv1.PodIPAllocation{
				UID: uid,
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
		})

		It("inputs nil Endpoint", func() {
			allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic2, nil)
			Expect(allocation).To(BeNil())
		})

		It("retrieves the IP allocation but the current record is nil", func() {
			allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic2, endpointT)
			Expect(allocation).To(BeNil())
		})

		It("retrieves non-existent current IP allocation", func() {
			endpointT.Status.Current = allocationT

			allocation := workloadendpointmanager.RetrieveIPAllocation(string(uuid.NewUUID()), nic2, endpointT)
			Expect(allocation).To(BeNil())
		})

		It("retrieves the current IP allocation", func() {
			endpointT.Status.Current = allocationT

			allocation := workloadendpointmanager.RetrieveIPAllocation(uid, nic2, endpointT)
			Expect(allocation).To(Equal(allocationT))
		})
	})
})
