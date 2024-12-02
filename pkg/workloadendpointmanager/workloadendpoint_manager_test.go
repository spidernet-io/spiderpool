// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/ptr"
	kubevirtv1 "kubevirt.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var _ = Describe("WorkloadEndpointManager", Label("workloadendpoint_manager_test"), func() {
	Describe("New WorkloadEndpointManager", func() {
		It("inputs nil client", func() {
			manager, err := workloadendpointmanager.NewWorkloadEndpointManager(
				nil,
				fakeAPIReader,
				true,
				true,
			)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil API reader", func() {
			manager, err := workloadendpointmanager.NewWorkloadEndpointManager(
				fakeClient,
				nil,
				true,
				true,
			)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test WorkloadEndpointManager's method", func() {
		var ctx context.Context

		var count uint64
		var namespace string
		var endpointName string
		var labels map[string]string
		var endpointT *spiderpoolv2beta1.SpiderEndpoint

		BeforeEach(func() {
			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			namespace = "default"
			endpointName = fmt.Sprintf("endpoint-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			endpointT = &spiderpoolv2beta1.SpiderEndpoint{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderEndpoint,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      endpointName,
					Namespace: namespace,
					Labels:    labels,
				},
				Status: spiderpoolv2beta1.WorkloadEndpointStatus{
					Current: spiderpoolv2beta1.PodIPAllocation{},
				},
			}
		})

		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: ptr.To(int64(0)),
				PropagationPolicy:  &policy,
			}

			err := fakeClient.Delete(ctx, endpointT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    constant.SpiderpoolAPIGroup,
					Version:  constant.SpiderpoolAPIVersion,
					Resource: "spiderendpoints",
				},
				endpointT.Namespace,
				endpointT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetEndpointByName", func() {
			It("gets non-existent Endpoint", func() {
				endpoint, err := endpointManager.GetEndpointByName(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(endpoint).To(BeNil())
			})

			It("gets an existing Endpoint through cache", func() {
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpoint, err := endpointManager.GetEndpointByName(ctx, namespace, endpointName, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint).NotTo(BeNil())
				Expect(endpoint).To(Equal(endpointT))
			})

			It("gets an existing Endpoint through API Server", func() {
				err := tracker.Add(endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpoint, err := endpointManager.GetEndpointByName(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint).NotTo(BeNil())
				Expect(endpoint).To(Equal(endpointT))
			})
		})

		Describe("ListEndpoints", func() {
			It("failed to list Endpoints due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeAPIReader, "List", constant.ErrUnknown)
				defer patches.Reset()

				err := tracker.Add(endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(endpointList).To(BeNil())
			})

			It("lists all Endpoints through cache", func() {
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointList.Items).NotTo(BeEmpty())

				hasEndpoint := false
				for _, endpoint := range endpointList.Items {
					if endpoint.Name == endpointName {
						hasEndpoint = true
						break
					}
				}
				Expect(hasEndpoint).To(BeTrue())
			})

			It("lists all Endpoints through API Server", func() {
				err := tracker.Add(endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointList.Items).NotTo(BeEmpty())

				hasEndpoint := false
				for _, endpoint := range endpointList.Items {
					if endpoint.Name == endpointName {
						hasEndpoint = true
						break
					}
				}
				Expect(hasEndpoint).To(BeTrue())
			})

			It("filters results by Namespace", func() {
				err := tracker.Add(endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, constant.IgnoreCache, client.MatchingFields{metav1.ObjectNameField: endpointName})
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointList.Items).NotTo(BeEmpty())

				hasEndpoint := false
				for _, endpoint := range endpointList.Items {
					if endpoint.Name == endpointName {
						hasEndpoint = true
						break
					}
				}
				Expect(hasEndpoint).To(BeTrue())
			})

			It("filters results by label selector", func() {
				err := tracker.Add(endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, constant.IgnoreCache, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointList.Items).NotTo(BeEmpty())

				hasEndpoint := false
				for _, endpoint := range endpointList.Items {
					if endpoint.Name == endpointName {
						hasEndpoint = true
						break
					}
				}
				Expect(hasEndpoint).To(BeTrue())
			})

			It("filters results by field selector", func() {
				err := tracker.Add(endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, constant.IgnoreCache, client.MatchingFields{metav1.ObjectNameField: endpointName})
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointList.Items).NotTo(BeEmpty())

				hasEndpoint := false
				for _, endpoint := range endpointList.Items {
					if endpoint.Name == endpointName {
						hasEndpoint = true
						break
					}
				}
				Expect(hasEndpoint).To(BeTrue())
			})
		})

		Describe("DeleteEndpoint", func() {
			It("deletes non-existent Endpoint", func() {
				err := endpointManager.DeleteEndpoint(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes an existing Endpoint", func() {
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.DeleteEndpoint(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("RemoveFinalizer", func() {
			It("inputs nil Endpoint", func() {
				err := endpointManager.RemoveFinalizer(ctx, nil)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("removes the finalizer that does not exit on the Endpoint", func() {
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.RemoveFinalizer(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to patch Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Patch", constant.ErrUnknown)
				defer patches.Reset()

				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)

				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.RemoveFinalizer(ctx, endpointT)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("removes the Endpoint's finalizer", func() {
				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)

				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.RemoveFinalizer(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv2beta1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: endpointName}, &endpoint)
				Expect(err).NotTo(HaveOccurred())

				contains := controllerutil.ContainsFinalizer(&endpoint, constant.SpiderFinalizer)
				Expect(contains).To(BeFalse())
			})
		})

		Describe("PatchIPAllocationResults", func() {
			var podT *corev1.Pod

			BeforeEach(func() {
				podT = &corev1.Pod{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Pod",
						APIVersion: corev1.SchemeGroupVersion.String(),
					},
					ObjectMeta: metav1.ObjectMeta{
						Name:      endpointName,
						Namespace: namespace,
						UID:       uuid.NewUUID(),
					},
					Spec: corev1.PodSpec{
						NodeName: "node",
					},
				}
			})

			It("inputs nil Pod", func() {
				err := endpointManager.PatchIPAllocationResults(ctx, []*spiderpooltypes.AllocationResult{}, nil, nil, spiderpooltypes.PodTopController{}, false)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("failed to set ownerReference to Pod due to some unknown errors", func() {
				patches := gomonkey.ApplyFuncReturn(controllerutil.SetOwnerReference, constant.ErrUnknown)
				defer patches.Reset()

				err := endpointManager.PatchIPAllocationResults(ctx, []*spiderpooltypes.AllocationResult{}, nil, podT, spiderpooltypes.PodTopController{}, false)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("failed to create Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Create", constant.ErrUnknown)
				defer patches.Reset()

				err := endpointManager.PatchIPAllocationResults(ctx, []*spiderpooltypes.AllocationResult{}, nil, podT, spiderpooltypes.PodTopController{}, false)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("creates Endpoint for orphan Pod", func() {
				err := endpointManager.PatchIPAllocationResults(
					ctx,
					[]*spiderpooltypes.AllocationResult{},
					nil,
					podT,
					spiderpooltypes.PodTopController{
						AppNamespacedName: spiderpooltypes.AppNamespacedName{
							APIVersion: corev1.SchemeGroupVersion.String(),
							Kind:       constant.KindPod,
							Namespace:  podT.Namespace,
							Name:       podT.Name,
						},
						UID: podT.UID,
						APP: podT,
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv2beta1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: podT.Namespace, Name: podT.Name}, &endpoint)
				Expect(err).NotTo(HaveOccurred())

				owner := endpoint.GetOwnerReferences()[0]
				Expect(owner.UID).To(Equal(podT.GetUID()))
				Expect(controllerutil.ContainsFinalizer(&endpoint, constant.SpiderFinalizer))
			})

			It("creates Endpoint for StatefulSet Pod", func() {
				err := endpointManager.PatchIPAllocationResults(
					ctx,
					[]*spiderpooltypes.AllocationResult{},
					nil,
					podT,
					spiderpooltypes.PodTopController{
						AppNamespacedName: spiderpooltypes.AppNamespacedName{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       constant.KindStatefulSet,
							Namespace:  namespace,
							Name:       fmt.Sprintf("%s-sts", endpointName),
						},
						UID: uuid.NewUUID(),
						APP: &appsv1.StatefulSet{},
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv2beta1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: podT.Namespace, Name: podT.Name}, &endpoint)
				Expect(err).NotTo(HaveOccurred())

				owners := endpoint.GetOwnerReferences()
				Expect(owners).To(BeEmpty())
				Expect(controllerutil.ContainsFinalizer(&endpoint, constant.SpiderFinalizer))
			})

			It("creates Endpoint for KubeVirt Pod", func() {
				vmiName := fmt.Sprintf("%s-vm", endpointName)

				err := endpointManager.PatchIPAllocationResults(
					ctx,
					[]*spiderpooltypes.AllocationResult{},
					nil,
					podT,
					spiderpooltypes.PodTopController{
						AppNamespacedName: spiderpooltypes.AppNamespacedName{
							APIVersion: kubevirtv1.SchemeGroupVersion.String(),
							Kind:       constant.KindKubevirtVMI,
							Namespace:  namespace,
							Name:       vmiName,
						},
						UID: uuid.NewUUID(),
						APP: &appsv1.StatefulSet{},
					},
					false,
				)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv2beta1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: podT.Namespace, Name: vmiName}, &endpoint)
				Expect(err).NotTo(HaveOccurred())

				owners := endpoint.GetOwnerReferences()
				Expect(owners).To(BeEmpty())
				Expect(controllerutil.ContainsFinalizer(&endpoint, constant.SpiderFinalizer))
			})

			It("patches IP allocation results with different Pod UID and node name", func() {
				podT.SetUID(uuid.NewUUID())
				endpointT.Status.Current.UID = string(uuid.NewUUID())

				err := endpointManager.PatchIPAllocationResults(ctx, []*spiderpooltypes.AllocationResult{}, endpointT, podT, spiderpooltypes.PodTopController{}, false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update the status of Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", constant.ErrUnknown)
				defer patches.Reset()

				uid := uuid.NewUUID()
				podT.SetUID(uid)
				endpointT.Status.Current.UID = string(uid)

				err := endpointManager.PatchIPAllocationResults(ctx, []*spiderpooltypes.AllocationResult{}, endpointT, podT, spiderpooltypes.PodTopController{}, false)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

		})

		Describe("ReallocateCurrentIPAllocation", func() {
			It("inputs nil Endpoint", func() {
				err := endpointManager.ReallocateCurrentIPAllocation(ctx, string(uuid.NewUUID()), "node1", "eth0", nil, false)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("re-allocates the current IP allocation with the same Pod UID", func() {
				uid := uuid.NewUUID()
				nodeName := "master"

				endpointT.Status.Current.UID = string(uid)
				endpointT.Status.Current.Node = nodeName

				err := endpointManager.ReallocateCurrentIPAllocation(ctx, string(uid), nodeName, "eth0", endpointT, false)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update the status of Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", constant.ErrUnknown)
				defer patches.Reset()

				endpointT.Status.Current.UID = string(uuid.NewUUID())

				err := endpointManager.ReallocateCurrentIPAllocation(ctx, string(uuid.NewUUID()), "node", "eth0", endpointT, false)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("updates the current IP allocation", func() {
				nic := "eth0"

				endpointT.Status.Current.UID = string(uuid.NewUUID())
				endpointT.Status.Current.Node = "old-node"
				endpointT.Status.Current.IPs = []spiderpoolv2beta1.IPAllocationDetail{
					{NIC: nic},
				}

				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				uid := string(uuid.NewUUID())
				nodeName := "new-node"

				err = endpointManager.ReallocateCurrentIPAllocation(ctx, uid, nodeName, nic, endpointT, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointT.Status.Current.UID).To(Equal(uid))
				Expect(endpointT.Status.Current.Node).To(Equal(nodeName))
				Expect(endpointT.Status.Current.IPs).To(HaveLen(1))
				Expect(endpointT.Status.Current.IPs[0].NIC).To(Equal(nic))
			})

			It("update the current IP allocation with new NIC name for empty nic field", func() {
				endpointT.Status.Current.UID = string(uuid.NewUUID())
				endpointT.Status.Current.Node = "old-node"
				endpointT.Status.Current.IPs = []spiderpoolv2beta1.IPAllocationDetail{{NIC: ""}}

				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				uid := string(uuid.NewUUID())
				nodeName := "new-node"
				nic := "eth0"

				err = endpointManager.ReallocateCurrentIPAllocation(ctx, uid, nodeName, nic, endpointT, true)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointT.Status.Current.UID).To(Equal(uid))
				Expect(endpointT.Status.Current.Node).To(Equal(nodeName))
				Expect(endpointT.Status.Current.IPs).To(HaveLen(1))
				Expect(endpointT.Status.Current.IPs[0].NIC).To(Equal(nic))
			})
		})

		Describe("UpdateAllocationNICName", func() {
			It("update the Endpoint status current IPs with new NIC name for empty nic filed", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", constant.ErrUnknown)
				defer patches.Reset()

				endpointT.Status.Current.UID = string(uuid.NewUUID())
				endpointT.Status.Current.Node = "old-node"
				endpointT.Status.Current.IPs = []spiderpoolv2beta1.IPAllocationDetail{
					{NIC: ""},
				}

				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				nic := "eth0"
				_, err = endpointManager.UpdateAllocationNICName(ctx, endpointT, nic)
				Expect(err).To(HaveOccurred())
			})

			It("failed to update the Endpoint status current IPs with new NIC name for empty nic filed", func() {
				endpointT.Status.Current.UID = string(uuid.NewUUID())
				endpointT.Status.Current.Node = "old-node"
				endpointT.Status.Current.IPs = []spiderpoolv2beta1.IPAllocationDetail{
					{NIC: ""},
				}

				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				nic := "eth0"
				podIPAllocation, err := endpointManager.UpdateAllocationNICName(ctx, endpointT, nic)
				Expect(err).NotTo(HaveOccurred())
				Expect(podIPAllocation.IPs).To(HaveLen(1))
				Expect(podIPAllocation.IPs[0].NIC).To(Equal(nic))
			})

			It("just return the PodIPAllocation due to the same NIC name", func() {
				nic := "eth0"
				endpointT.Status.Current.UID = string(uuid.NewUUID())
				endpointT.Status.Current.Node = "old-node"
				endpointT.Status.Current.IPs = []spiderpoolv2beta1.IPAllocationDetail{
					{NIC: nic},
				}

				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				podIPAllocation, err := endpointManager.UpdateAllocationNICName(ctx, endpointT, nic)
				Expect(err).NotTo(HaveOccurred())
				Expect(podIPAllocation.IPs).To(HaveLen(1))
				Expect(podIPAllocation.IPs[0].NIC).To(Equal(nic))
			})
		})

		Describe("ReleaseEndpointIPs", func() {
			It("failed to release SpiderEndpoint IPs due to mismatch the PodUID", func() {
				endpointT.Status.Current.UID = string(uuid.NewUUID())
				_, err := endpointManager.ReleaseEndpointIPs(ctx, endpointT, string(uuid.NewUUID()))
				Expect(err).To(HaveOccurred())
			})

			It("no SpiderEndpoint recorded IPs", func() {
				podUID := string(uuid.NewUUID())

				endpointT.Status.Current.UID = podUID
				ipAllocationDetails, err := endpointManager.ReleaseEndpointIPs(ctx, endpointT, podUID)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipAllocationDetails).To(HaveLen(0))
			})

			It("failed to update SpiderEndpoint", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", constant.ErrUnknown)
				defer patches.Reset()

				podUID := string(uuid.NewUUID())

				endpointT.Status.Current.UID = podUID
				endpointT.Status.Current.IPs = []spiderpoolv2beta1.IPAllocationDetail{
					{
						NIC:  "eth0",
						IPv4: ptr.To("172.10.2.3/16"),
					},
				}
				_, err := endpointManager.ReleaseEndpointIPs(ctx, endpointT, podUID)
				Expect(err).To(HaveOccurred())
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("release SpiderEndpoint recorded IPs successfully", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", nil)
				defer patches.Reset()

				podUID := string(uuid.NewUUID())

				endpointT.Status.Current.UID = podUID
				endpointT.Status.Current.IPs = []spiderpoolv2beta1.IPAllocationDetail{
					{
						NIC:  "eth0",
						IPv4: ptr.To("172.100.1.2/16"),
						IPv6: ptr.To("fd00:172:100::201/64"),
					},
					{
						NIC:  "net1",
						IPv4: ptr.To("172.200.1.2/16"),
						IPv6: ptr.To("fd00:172:200::201/64"),
					},
				}

				ipAllocationDetails, err := endpointManager.ReleaseEndpointIPs(ctx, endpointT, podUID)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointT.Status.Current.IPs).To(HaveLen(0))
				Expect(ipAllocationDetails).To(HaveLen(2))
			})
		})

		Describe("ReleaseEndpointAndFinalizer", func() {

			It("failed to release EndpointAndFinalizer due to getting non-existent Endpoint", func() {
				err := endpointManager.ReleaseEndpointAndFinalizer(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(err).To(BeNil())
			})

			It("should return an error if getting the endpoint fails with an unknown error", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Get", constant.ErrUnknown)
				defer patches.Reset()

				err := endpointManager.ReleaseEndpointAndFinalizer(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("should delete the endpoint if DeletionTimestamp is nil", func() {
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				patches := gomonkey.ApplyMethodReturn(fakeClient, "Delete", nil)
				defer patches.Reset()

				err = endpointManager.ReleaseEndpointAndFinalizer(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error if DeleteEndpoint fails", func() {
				patches := gomonkey.ApplyMethodReturn(endpointManager, "GetEndpointByName", endpointT, nil)
				defer patches.Reset()

				patchDelete := gomonkey.ApplyMethodReturn(endpointManager, "DeleteEndpoint", constant.ErrUnknown)
				defer patchDelete.Reset()

				err := endpointManager.ReleaseEndpointAndFinalizer(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("should remove the finalizer if the endpoint was successfully deleted", func() {
				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", nil)
				defer patches.Reset()

				err = endpointManager.ReleaseEndpointAndFinalizer(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should succeed to release finalizer when there is no error", func() {
				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)
				endpointT.DeletionTimestamp = &metav1.Time{Time: time.Now()}

				patches := gomonkey.ApplyMethodReturn(endpointManager, "GetEndpointByName", endpointT, nil)
				defer patches.Reset()

				patchRemoveFinalizer := gomonkey.ApplyMethodReturn(endpointManager, "RemoveFinalizer", nil)
				defer patchRemoveFinalizer.Reset()

				err := endpointManager.ReleaseEndpointAndFinalizer(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error if RemoveFinalizer fails", func() {
				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)
				endpointT.DeletionTimestamp = &metav1.Time{Time: time.Now()}

				patches := gomonkey.ApplyMethodReturn(endpointManager, "GetEndpointByName", endpointT, nil)
				defer patches.Reset()

				patchRemoveFinalizer := gomonkey.ApplyMethodReturn(endpointManager, "RemoveFinalizer", constant.ErrUnknown)
				defer patchRemoveFinalizer.Reset()

				err := endpointManager.ReleaseEndpointAndFinalizer(ctx, namespace, endpointName, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})
		})

		Describe("PatchEndpointAllocationIPs", func() {
			var endpointT *spiderpoolv2beta1.SpiderEndpoint
			var newEndpointIPs []spiderpoolv2beta1.IPAllocationDetail

			BeforeEach(func() {
				endpointT = &spiderpoolv2beta1.SpiderEndpoint{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-endpoint",
						Namespace: "default",
					},
					Status: spiderpoolv2beta1.WorkloadEndpointStatus{
						Current: spiderpoolv2beta1.PodIPAllocation{
							IPs: []spiderpoolv2beta1.IPAllocationDetail{
								{NIC: "eth0", IPv4: ptr.To("192.168.1.1/24")},
							},
						},
					},
				}

				newEndpointIPs = []spiderpoolv2beta1.IPAllocationDetail{
					{NIC: "eth0", IPv4: ptr.To("192.168.1.2/24")},
					{NIC: "eth1", IPv4: ptr.To("192.168.1.3/24")},
				}
			})

			It("successfully patches the SpiderEndpoint with new IPs", func() {
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.PatchEndpointAllocationIPs(ctx, endpointT, newEndpointIPs)
				Expect(err).NotTo(HaveOccurred())

				var updatedEndpoint spiderpoolv2beta1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: endpointT.Namespace, Name: endpointT.Name}, &updatedEndpoint)
				Expect(err).NotTo(HaveOccurred())
				Expect(updatedEndpoint.Status.Current.IPs).To(Equal(newEndpointIPs))
			})

			It("fails to patch the SpiderEndpoint due to update error", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", constant.ErrUnknown)
				defer patches.Reset()

				err := endpointManager.PatchEndpointAllocationIPs(ctx, endpointT, newEndpointIPs)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})
		})
	})
})
