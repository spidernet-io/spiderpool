// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/moby/moby/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	"github.com/spidernet-io/spiderpool/pkg/types"

	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("IPPoolManager_test", Label("ippool_manager_test"), func() {
	Describe("New IPPoolManager", func() {
		It("sets default config", func() {
			manager, err := ippoolmanager.NewIPPoolManager(ippoolmanager.IPPoolManagerConfig{}, fakeClient, rIPManagerMock)
			Expect(err).NotTo(HaveOccurred())
			Expect(manager).NotTo(BeNil())
		})

		It("inputs nil client", func() {
			manager, err := ippoolmanager.NewIPPoolManager(ippoolmanager.IPPoolManagerConfig{}, nil, rIPManager)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil reserved-IP manager", func() {
			manager, err := ippoolmanager.NewIPPoolManager(ippoolmanager.IPPoolManagerConfig{}, fakeClient, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test IPPoolManager's method", func() {
		var count uint64
		var namespace string
		var podName string
		var ipPoolName string
		var ipPoolT *spiderpoolv1.SpiderIPPool
		var labels map[string]string

		BeforeEach(func() {
			atomic.AddUint64(&count, 1)
			ipPoolName = fmt.Sprintf("IPPool-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			ipPoolT = &spiderpoolv1.SpiderIPPool{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderIPPoolKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   ipPoolName,
					Labels: labels,
				},
				Spec: spiderpoolv1.IPPoolSpec{},
			}
		})
		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
				PropagationPolicy:  &policy,
			}

			ctx := context.TODO()
			err := fakeClient.Delete(ctx, ipPoolT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetIPPoolByName", func() {
			It("gets non-existent IPPool", func() {
				ctx := context.TODO()
				ippool, err := ipPoolManager.GetIPPoolByName(ctx, ipPoolName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(ippool).To(BeNil())
			})

			It("gets an existing IPPool", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ippool, err := ipPoolManager.GetIPPoolByName(ctx, ipPoolName)
				Expect(err).NotTo(HaveOccurred())
				Expect(ippool).NotTo(BeNil())
				Expect(ippool).To(Equal(ipPoolT))
			})
		})

		Describe("ListIPPools", func() {
			It("failed to list IPPools due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(ipPoolList).To(BeNil())
			})

			It("lists all IPPools", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolList.Items).NotTo(BeEmpty())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeTrue())
			})

			It("filters results by label selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolList.Items).NotTo(BeEmpty())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeTrue())
			})

			It("filters results by field selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx, client.MatchingFields{metav1.ObjectNameField: ipPoolName})
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolList.Items).NotTo(BeEmpty())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeTrue())
			})
		})

		Describe("AllocateIP", func() {
			var podT *corev1.Pod

			BeforeEach(func() {
				ipPoolT.SetUID(uuid.NewUUID())
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Vlan = pointer.Int64(0)
				ipPoolT.Spec.Subnet = "172.19.41.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.19.41.2",
						"172.19.41.3-172.19.41.100",
					}...,
				)

				podT = &corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      podName,
						Namespace: namespace,
						UID:       uuid.NewUUID(),
					},
					Spec: corev1.PodSpec{
						NodeName: "node",
					},
				}

			})

			It("inputs non-existent IPPool name", func() {
				ctx := context.TODO()
				ipPool, err := ipPoolManager.AllocateIP(ctx, ipPoolName, stringid.GenerateRandomID(), constant.ClusterDefaultInterfaceName, podT, types.PodTopController{})
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(ipPool).To(BeNil())
			})

			It("inputs nil Pod", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPool, err := ipPoolManager.AllocateIP(ctx, ipPoolName, stringid.GenerateRandomID(), constant.ClusterDefaultInterfaceName, nil, types.PodTopController{})
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
				Expect(ipPool).To(BeNil())
			})

			It("allocate IP but IPPool's IPs is empty", func() {
				ipPoolT.Spec.IPs = []string{}
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPool, err := ipPoolManager.AllocateIP(ctx, ipPoolName, stringid.GenerateRandomID(), constant.ClusterDefaultInterfaceName, podT, types.PodTopController{})
				Expect(err).To(MatchError(constant.ErrIPUsedOut))
				Expect(ipPool).To(BeNil())
			})

			It("failed to allocate IP due to some unknown errors", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()
				ipPool, err := ipPoolManager.AllocateIP(ctx, ipPoolName, stringid.GenerateRandomID(), constant.ClusterDefaultInterfaceName, podT, types.PodTopController{})
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(ipPool).To(BeNil())
			})

			PIt("runs out of retries to allocate IP , but conflicts still occur", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", apierrors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil))
				defer patches.Reset()

				controllerutil.AddFinalizer(ipPoolT, constant.SpiderFinalizer)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPool, err := ipPoolManager.AllocateIP(ctx, ipPoolName, stringid.GenerateRandomID(), constant.ClusterDefaultInterfaceName, podT, types.PodTopController{})
				Expect(err).To(MatchError(constant.ErrRetriesExhausted))
				Expect(ipPool).To(BeNil())
			})

			It("passes", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				ipPool, err := ipPoolManager.AllocateIP(ctx, ipPoolName, stringid.GenerateRandomID(), constant.ClusterDefaultInterfaceName, podT, types.PodTopController{})
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPool).NotTo(BeNil())
				Expect(ipPool.IPPool).To(Equal(ipPoolName))
				Expect(*ipPool.Nic).To(Equal(constant.ClusterDefaultInterfaceName))
			})
		})

		Describe("ReleaseIP", func() {
			var containerID, diffContainerID string
			var ipNumber, diffIPNumber string
			var diffIPAndCID []types.IPAndCID

			BeforeEach(func() {
				ipNumber = "172.19.45.2"
				diffIPNumber = "172.19.45.3"
				ipPoolT.Spec.Subnet = "172.19.45.0/24"
				containerID = stringid.GenerateRandomID()
				diffContainerID = stringid.GenerateRandomID()

				ipPoolT.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{}
				allocation := spiderpoolv1.PoolIPAllocation{
					ContainerID: containerID,
				}
				ipPoolT.Status.AllocatedIPs[ipNumber] = allocation

				diffIPAndCID = []types.IPAndCID{
					{
						IP:          ipNumber,
						ContainerID: containerID,
					},
				}
			})

			It("inputs non-existent IPPool name", func() {
				ctx := context.TODO()
				err := ipPoolManager.ReleaseIP(ctx, ipPoolName, diffIPAndCID)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})

			It("inputs nil ipAndCID", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.ReleaseIP(ctx, ipPoolName, nil)
				Expect(err).To(BeNil())
			})

			When("The status IP of the IPPool is the same as the ipAndCIDs[x].IP", func() {
				It("different container ID", func() {
					diffIPAndCID[0].ContainerID = diffContainerID

					ctx := context.TODO()
					err := fakeClient.Create(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())

					err = ipPoolManager.ReleaseIP(ctx, ipPoolName, diffIPAndCID)
					Expect(err).To(BeNil())
				})
			})

			When("The status IP of IPPool is different from ipAndCIDs[x].IP", func() {
				It("different IP", func() {
					ipPoolT.Status.AllocatedIPs[ipNumber] = spiderpoolv1.PoolIPAllocation{
						ContainerID: containerID,
					}
					diffIPNumberCID := []types.IPAndCID{
						{
							IP: diffIPNumber,
						},
					}
					ctx := context.TODO()
					err := fakeClient.Create(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())

					err = ipPoolManager.ReleaseIP(ctx, ipPoolName, diffIPNumberCID)
					Expect(err).To(BeNil())
				})
			})

			It("failed to release IP due to some unknown errors", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				err = ipPoolManager.ReleaseIP(ctx, ipPoolName, diffIPAndCID)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			PIt("runs out of retries to release IP , but conflicts still occur", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", apierrors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil))
				defer patches.Reset()

				controllerutil.AddFinalizer(ipPoolT, constant.SpiderFinalizer)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.ReleaseIP(ctx, ipPoolName, diffIPAndCID)
				Expect(err).To(MatchError(constant.ErrRetriesExhausted))
			})

			It("passes", func() {
				diffIPAndCID[0].ContainerID = containerID

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.ReleaseIP(ctx, ipPoolName, diffIPAndCID)
				Expect(err).To(BeNil())
			})
		})

		Describe("UpdateAllocatedIPs", func() {
			var containerID, diffContainerID string
			var ipNumber, diffIPNumber string
			var diffIPAndCID []types.IPAndCID

			BeforeEach(func() {
				ipNumber = "172.19.40.2"
				diffIPNumber = "172.19.40.3"
				containerID = stringid.GenerateRandomID()
				diffContainerID = stringid.GenerateRandomID()

				ipPoolT.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{}
				allocation := spiderpoolv1.PoolIPAllocation{
					ContainerID: containerID,
				}
				ipPoolT.Status.AllocatedIPs[ipNumber] = allocation

				diffIPAndCID = []types.IPAndCID{
					{
						IP:          ipNumber,
						ContainerID: diffContainerID,
					},
				}
			})

			It("inputs non-existent IPPool name", func() {
				ctx := context.TODO()
				err := ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, diffIPAndCID)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})

			It("inputs nil resource", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("test updating the same IP and container ID", func() {
				ipPoolT.Status.AllocatedIPs[ipNumber] = spiderpoolv1.PoolIPAllocation{
					ContainerID: containerID,
				}
				sameIPAndContainerID := []types.IPAndCID{
					{
						IP:          ipNumber,
						ContainerID: containerID,
					},
				}
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, sameIPAndContainerID)
				Expect(err).To(BeNil())
			})

			It("test updating the diff IP Number", func() {
				ipPoolT.Status.AllocatedIPs[ipNumber] = spiderpoolv1.PoolIPAllocation{
					ContainerID: containerID,
				}

				diffIPNumberCID := []types.IPAndCID{
					{
						IP: diffIPNumber,
					},
				}
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, diffIPNumberCID)
				Expect(err).To(BeNil())
			})

			It("failed to update the status of IPPool AllocatedIPs due to some unknown errors", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, diffIPAndCID)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("passes", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateAllocatedIPs(ctx, ipPoolName, diffIPAndCID)
				Expect(err).To(BeNil())
			})
		})

		Describe("DeleteAllIPPools", func() {
			var deleteAllOfOption client.DeleteAllOfOption

			BeforeEach(func() {
				policy := metav1.DeletePropagationForeground
				deleteAllOfOption = &client.DeleteAllOfOptions{
					DeleteOptions: client.DeleteOptions{
						GracePeriodSeconds: pointer.Int64(0),
						PropagationPolicy:  &policy,
					},
				}
			})

			It("inputs nil IPPool", func() {
				ctx := context.TODO()
				err := ipPoolManager.DeleteAllIPPools(ctx, nil)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("Delete All IPPools", func() {
				newIPPoolT := ipPoolT.DeepCopy()
				newIPPoolT.SetUID(uuid.NewUUID())
				newIPPoolT.Name = fmt.Sprintf("newIPPool-%v", count)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				err = fakeClient.Create(ctx, newIPPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.DeleteAllIPPools(ctx, ipPoolT, deleteAllOfOption)
				Expect(err).NotTo(HaveOccurred())

				ipPoolList, err := ipPoolManager.ListIPPools(ctx)
				Expect(err).NotTo(HaveOccurred())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName || ipPool.Name == newIPPoolT.Name {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeFalse())
			})

			It("delete all ippools by label selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.DeleteAllIPPools(ctx, ipPoolT, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				ipPoolList, err := ipPoolManager.ListIPPools(ctx)
				Expect(err).NotTo(HaveOccurred())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeFalse())
			})

			It("delete all ippools by field selector", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.DeleteAllIPPools(ctx, ipPoolT, client.MatchingFields{metav1.ObjectNameField: ipPoolName})
				Expect(err).NotTo(HaveOccurred())
				ipPoolList, err := ipPoolManager.ListIPPools(ctx)
				Expect(err).NotTo(HaveOccurred())

				hasIPPool := false
				for _, ipPool := range ipPoolList.Items {
					if ipPool.Name == ipPoolName {
						hasIPPool = true
						break
					}
				}
				Expect(hasIPPool).To(BeFalse())
			})
		})

		Describe("UpdateDesiredIPNumber", func() {
			var ipNum int

			BeforeEach(func() {
				ipNum = 1
			})

			It("inputs nil IPPool ", func() {
				ctx := context.TODO()
				err := ipPoolManager.UpdateDesiredIPNumber(ctx, nil, ipNum)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("inputs nil AutoDesiredIPCount", func() {
				ipPoolT.Status.AutoDesiredIPCount = nil
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateDesiredIPNumber(ctx, ipPoolT, ipNum)
				Expect(err).NotTo(HaveOccurred())
				Expect(*ipPoolT.Status.AutoDesiredIPCount).To(Equal(int64(ipNum)))
			})

			It("Enter an ip number equal to the autoDesiredIPCount ", func() {
				var autoDesiredIPCount int = 2
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(2)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateDesiredIPNumber(ctx, ipPoolT, autoDesiredIPCount)
				Expect(err).NotTo(HaveOccurred())
				Expect(*ipPoolT.Status.AutoDesiredIPCount).To(Equal(int64(autoDesiredIPCount)))
			})

			It("IPPool status AutoDesiredIPCount is nil", func() {
				ipPoolT.Status.AutoDesiredIPCount = nil

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateDesiredIPNumber(ctx, ipPoolT, ipNum)
				Expect(err).NotTo(HaveOccurred())
				Expect(*ipPoolT.Status.AutoDesiredIPCount).To(Equal(int64(ipNum)))
			})

			It("failed to update the status of IPPool AutoDesiredIPCount due to some unknown errors", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				errInfo := fmt.Errorf("failed to update IPPool '%s' auto desired IP count to %d : unknown", ipPoolName, ipNum)
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				err = ipPoolManager.UpdateDesiredIPNumber(ctx, ipPoolT, ipNum)
				Expect(err).To(MatchError(errInfo))
			})

			It("passes", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolManager.UpdateDesiredIPNumber(ctx, ipPoolT, ipNum)
				Expect(err).NotTo(HaveOccurred())
				Expect(*ipPoolT.Status.AutoDesiredIPCount).To(Equal(int64(ipNum)))
			})
		})
	})
})
