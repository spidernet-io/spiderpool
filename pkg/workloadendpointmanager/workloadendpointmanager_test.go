// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const namespace string = "group"
const wename string = "workload"
const containerID string = "dummy"

var we *spiderpoolv1.WorkloadEndpoint
var ipAllocation *spiderpoolv1.PodIPAllocation
var HispodAllo []spiderpoolv1.PodIPAllocation
var nodename string

var _ = Describe("Workloadendpointmanager", Label("unittest", "WorkloadEndpoint"), func() {
	BeforeEach(func() {
		// Initialize parameter
		nodename = "v1" + "node"

		// Initialize PodIPAllocation
		ipAllocation = &spiderpoolv1.PodIPAllocation{
			ContainerID: containerID,
			Node:        &nodename,
			IPs: []spiderpoolv1.IPAllocationDetail{
				{
					NIC:         "eth0",
					IPv4:        new(string),
					IPv4Pool:    new(string),
					IPv4Gateway: new(string),
					IPv6:        new(string),
					IPv6Pool:    new(string),
					IPv6Gateway: new(string),
				},
			},
		}
		*ipAllocation.IPs[0].IPv4 = "127.1.0.5/24"
		*ipAllocation.IPs[0].IPv4Pool = "pool1"
		*ipAllocation.IPs[0].IPv4Gateway = "127.1.0.1"
		*ipAllocation.IPs[0].IPv6 = "2001:0db8:3c4d:0015:0000:0000:1a2f:1a2b/24"
		*ipAllocation.IPs[0].IPv6Pool = "pool1"
		*ipAllocation.IPs[0].IPv6Gateway = "127.2.3.1"

		ipAllocationV2 := &spiderpoolv1.PodIPAllocation{
			ContainerID: containerID,
			Node:        &nodename,
			IPs: []spiderpoolv1.IPAllocationDetail{
				{
					NIC:         "eth1",
					IPv4:        new(string),
					IPv4Pool:    new(string),
					IPv4Gateway: new(string),
					IPv6:        new(string),
					IPv6Pool:    new(string),
					IPv6Gateway: new(string),
				},
			},
		}
		*ipAllocationV2.IPs[0].IPv4 = "127.1.0.6/24"
		*ipAllocationV2.IPs[0].IPv4Pool = "pool2"
		*ipAllocationV2.IPs[0].IPv4Gateway = "127.1.0.2"
		*ipAllocationV2.IPs[0].IPv6 = "3001:0db8:3c4d:0025:0000:0000:1a2f:1a2b/24"
		*ipAllocationV2.IPs[0].IPv6Pool = "pool2"
		*ipAllocationV2.IPs[0].IPv6Gateway = "127.2.3.2"

		//HispodAllo := []spiderpoolv1.PodIPAllocation{podAllo, podAllo}
		HispodAllo = []spiderpoolv1.PodIPAllocation{*ipAllocation, *ipAllocationV2}
		//*podAllo.Node = nodename

		// Initialize WorkloadEndpoint
		we = &spiderpoolv1.WorkloadEndpoint{
			ObjectMeta: metav1.ObjectMeta{
				Name:      wename,
				Namespace: namespace,
			},
			Status: spiderpoolv1.WorkloadEndpointStatus{
				Current: ipAllocation,
				History: HispodAllo,
			},
		}
		// clean up the client
		err = k8sClient.DeleteAllOf(ctx, we)
		Expect(err).ShouldNot(HaveOccurred())

		err = k8sClient.Create(ctx, we)
		Expect(err).ShouldNot(HaveOccurred())

	})

	Context("CURD operation in WorkloadEndpoint ", func() {
		var newAllocation *spiderpoolv1.PodIPAllocation
		BeforeEach(func() {
			newAllocation = &spiderpoolv1.PodIPAllocation{
				ContainerID: containerID,
				Node:        &nodename,
				IPs: []spiderpoolv1.IPAllocationDetail{
					{
						NIC: "eth0",
					},
					{
						NIC: "eth1",
					},
				},
			}

			// update WorkloadEndpoint current
			we.Status.Current = newAllocation
			err = k8sClient.Status().Update(ctx, we)
		})

		DescribeTable("retrieve IPAllocation from client", func(createobj func(), nic string, includeHistory bool) {
			// create obj in the Kubernetes cluster.
			createobj()
			// retrieve IPAllocation
			ipallocate, succ, err := weManager.RetriveIPAllocation(ctx, namespace, wename, containerID, nic, includeHistory)
			Expect(err).ShouldNot(HaveOccurred())

			// check ipallocate.containerID
			Expect(containerID).Should(Equal(ipallocate.ContainerID))

			// check ipallocate.Node
			Expect(nodename).Should(Equal(*ipallocate.Node))

			// check the return bool value
			if !includeHistory {
				Expect(succ).Should(Equal(true))
			} else {
				Expect(succ).Should(Equal(false))
			}

		},
			Entry("passing the WorkloadEndpoint with Current status", func() {
				we.Status.History = nil
				err = k8sClient.Status().Update(ctx, we)
				Expect(err).ShouldNot(HaveOccurred())
			}, "eth0", false),
			Entry("passing the WorkloadEndpoint with history", func() {
				//  clean up the Current IPAllocation
				err = weManager.ClearCurrentIPAllocation(ctx, namespace, wename, containerID)
				Expect(err).ShouldNot(HaveOccurred())
			}, "eth1", true))

		DescribeTable("MarkIPAllocation", func(setConfig func(), v1name, v1NameSpace string) {
			setConfig()
			err = weManager.MarkIPAllocation(ctx, nodename, v1NameSpace, v1name, containerID)
			Expect(err).ShouldNot(HaveOccurred())

			// collect newWE history IPs and classify them with each pool name.
			newWE, err := weManager.GetEndpointByName(ctx, v1NameSpace, v1name)
			Expect(err).ShouldNot(HaveOccurred())

			// Check WorkloadEndpoint.name
			Expect(v1name).Should(Equal(newWE.Name))

			// Check WorkloadEndpoint.namespace
			Expect(v1NameSpace).Should(Equal(newWE.Namespace))

		},
			Entry("It will create a new endpoint if WorkloadEndpoint not be found", func() {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "v1pod",
						Namespace: "v1group",
					},
				}
				err = k8sClient.Create(ctx, pod)
				Expect(err).ShouldNot(HaveOccurred())

			}, "v1pod", "v1group"),
			Entry("It will return nil if the Current IPAllocation of WorkloadEndpoint is nil ", func() {
				//  clean up the Current IPAllocation
				err = weManager.ClearCurrentIPAllocation(ctx, namespace, wename, containerID)
				Expect(err).ShouldNot(HaveOccurred())
			}, "workload", "group"),
			Entry("It will success if WorkloadEndpoint registered normally", func() {}, "workload", "group"),
		)

		DescribeTable("PatchIPAllocation", func(isStatusNil, isCidMatch, isMerged bool, getAllocation func() *spiderpoolv1.PodIPAllocation) {
			podAllocate := getAllocation()
			err = weManager.PatchIPAllocation(ctx, namespace, wename, podAllocate)

			// Check the return error
			if isStatusNil {
				Expect(err.Error()).To(ContainSubstring("unmarked"))
				return
			} else if isCidMatch {
				Expect(err.Error()).To(ContainSubstring("mismarked"))
				return
			} else {
				Expect(err).ShouldNot(HaveOccurred())
			}

			newWE, err := weManager.GetEndpointByName(ctx, namespace, wename)
			Expect(err).ShouldNot(HaveOccurred())

			if isMerged {
				// Check whether podAllocate merge into WorkloadEndpoint or not
				Expect(newWE.Status.Current.IPs[0]).To(Equal(podAllocate.IPs[0]))
			} else {
				// Check whether podAllocate pathc into WorkloadEndpoint or not
				Expect(newWE.Status.Current.IPs[2]).To(Equal(podAllocate.IPs[0]))

			}

		},
			Entry("It will merge into WorkloadEndpoint when passing with the match PodIPAllocation", false, false, true, func() *spiderpoolv1.PodIPAllocation {
				return ipAllocation
			}),
			Entry("It will patch into WorkloadEndpoint when passing with the mismatch PodIPAllocation", false, false, false, func() *spiderpoolv1.PodIPAllocation {
				ipAllocation.IPs[0].NIC = "eth2"
				return ipAllocation
			}),
			Entry("It will fail when passing with empty Current IPAllocation", true, false, false, func() *spiderpoolv1.PodIPAllocation {
				err = weManager.ClearCurrentIPAllocation(ctx, namespace, wename, containerID)
				Expect(err).ShouldNot(HaveOccurred())
				return nil
			}),
			Entry("It will fail with mismatch CID", false, true, false, func() *spiderpoolv1.PodIPAllocation {
				ipAllocation := &spiderpoolv1.PodIPAllocation{
					ContainerID: "shadow",
					Node:        &nodename,
					IPs:         []spiderpoolv1.IPAllocationDetail{},
				}
				return ipAllocation
			}),
		)
	})

	Context("Testing functional functions", func() {
		var newSpace, newName string
		BeforeEach(func() {
			// Initialize parameter
			newSpace = "v1" + namespace
			newName = "v1" + wename
			nodename = "v2node"

			// Initialize WorkloadEndpoint
			we = &spiderpoolv1.WorkloadEndpoint{
				ObjectMeta: metav1.ObjectMeta{
					Name:      newName,
					Namespace: newSpace,
				},
				Status: spiderpoolv1.WorkloadEndpointStatus{
					Current: ipAllocation,
					History: HispodAllo,
				},
			}

			// clean up the client
			err = k8sClient.DeleteAllOf(ctx, we)
			Expect(err).ShouldNot(HaveOccurred())

			err = k8sClient.Create(ctx, we)
			Expect(err).ShouldNot(HaveOccurred())

		})

		It("Testing ListAllHistoricalIPs", func() {
			// classify history IPS with each pool name.
			icGroup, err := weManager.ListAllHistoricalIPs(ctx, newSpace, newName)
			Expect(err).ShouldNot(HaveOccurred())

			// icGroup will contain ippool in WorkloadEndpoint Current
			for _, currentAllocationDetail := range we.Status.Current.IPs {
				ipPool := *currentAllocationDetail.IPv4Pool
				ipPoolGroup, ok := icGroup[ipPool]
				Expect(ok).Should(BeTrue())

				// Check the ipPool in pPoolGroup belong to WorkloadEndpoint Current
				for _, ipandCID := range ipPoolGroup {
					exist, err := weManager.IsIPBelongWEPCurrent(ctx, newSpace, newName, ipandCID.IP)
					Expect(exist).Should(BeTrue())
					Expect(err).ShouldNot(HaveOccurred())
				}
			}
		})

		It("Testing RemoveFinalizer", func() {
			// Add Finalizer
			controllerutil.AddFinalizer(we, constant.SpiderFinalizer)
			err = k8sClient.Status().Update(ctx, we)
			Expect(err).ShouldNot(HaveOccurred())

			// Remove Finalizer from WorkloadEndpoint
			err = weManager.RemoveFinalizer(ctx, newSpace, newName)
			Expect(err).ShouldNot(HaveOccurred())

			weRemoveFinalizer, err := weManager.GetEndpointByName(ctx, newSpace, newName)
			Expect(err).ShouldNot(HaveOccurred())

			// Check Finalizer whether to be removed
			exist := controllerutil.ContainsFinalizer(weRemoveFinalizer, constant.SpiderFinalizer)
			Expect(exist).Should(BeFalse())
		})

		DescribeTable("Testing CheckCurrentContainerID", func(isBroken, isNotContain bool, CID string) {
			// Clean up we.Status.History to break it
			if isBroken {
				we.Status.History = nil
				err = k8sClient.Status().Update(ctx, we)
				Expect(err).ShouldNot(HaveOccurred())
			}

			exist, err := weManager.CheckCurrentContainerID(ctx, newSpace, newName, CID)
			if exist {
				// Contained given string
				Expect(exist).Should(BeTrue())
				Expect(err).ShouldNot(HaveOccurred())
			} else if isNotContain {
				// Not contained given string
				Expect(exist).Should(BeFalse())
				Expect(err).ShouldNot(HaveOccurred())
			} else {
				// Data broken
				Expect(exist).Should(BeFalse())
				Expect(err.Error()).To(ContainSubstring("broken"))
			}

		},
			Entry("It will success while passing the ContainerID ", false, false, containerID),
			Entry("It will fail while passing the mismatch ContainerID", false, true, "shadow"),
			Entry("It will return error while data broken", true, false, containerID),
		)

	})

})
