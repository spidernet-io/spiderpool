// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	"github.com/golang/mock/gomock"
	"github.com/moby/moby/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	spiderpooltypes "github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var _ = Describe("WorkloadEndpointManager", Label("workloadendpoint_manager_test"), func() {
	Describe("New WorkloadEndpointManager", func() {
		It("sets default config", func() {
			manager, err := workloadendpointmanager.NewWorkloadEndpointManager(
				workloadendpointmanager.EndpointManagerConfig{},
				fakeClient,
				mockPodManager,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(manager).NotTo(BeNil())
		})

		It("inputs nil client", func() {
			manager, err := workloadendpointmanager.NewWorkloadEndpointManager(
				workloadendpointmanager.EndpointManagerConfig{},
				nil,
				mockPodManager,
			)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil Pod manager", func() {
			manager, err := workloadendpointmanager.NewWorkloadEndpointManager(
				workloadendpointmanager.EndpointManagerConfig{},
				fakeClient,
				nil,
			)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test WorkloadEndpointManager's method", func() {
		var count uint64
		var namespace string
		var endpointName string
		var labels map[string]string
		var endpointT *spiderpoolv1.SpiderEndpoint

		BeforeEach(func() {
			atomic.AddUint64(&count, 1)
			namespace = "default"
			endpointName = fmt.Sprintf("endpoint-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			endpointT = &spiderpoolv1.SpiderEndpoint{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderEndpointKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:      endpointName,
					Namespace: namespace,
					Labels:    labels,
				},
				Status: spiderpoolv1.WorkloadEndpointStatus{
					Current: &spiderpoolv1.PodIPAllocation{},
				},
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
			err := fakeClient.Delete(ctx, endpointT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetEndpointByName", func() {
			It("gets non-existent Endpoint", func() {
				ctx := context.TODO()
				endpoint, err := endpointManager.GetEndpointByName(ctx, namespace, endpointName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(endpoint).To(BeNil())
			})

			It("gets an existing Endpoint", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpoint, err := endpointManager.GetEndpointByName(ctx, namespace, endpointName)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint).NotTo(BeNil())

				Expect(endpoint).To(Equal(endpointT))
			})
		})

		Describe("ListEndpoints", func() {
			It("failed to list Endpoints due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(endpointList).To(BeNil())
			})

			It("lists all Endpoints", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx)
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
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, client.MatchingFields{metav1.ObjectNameField: endpointName})
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
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, client.MatchingLabels(labels))
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
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				endpointList, err := endpointManager.ListEndpoints(ctx, client.MatchingFields{metav1.ObjectNameField: endpointName})
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
				ctx := context.TODO()
				err := endpointManager.DeleteEndpoint(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("deletes an existing Endpoint", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.DeleteEndpoint(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("RemoveFinalizer", func() {
			It("removes the finalizer for non-existent Endpoint", func() {
				ctx := context.TODO()
				err := endpointManager.RemoveFinalizer(ctx, namespace, endpointName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("removes the finalizer that does not exit on the Endpoint", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.RemoveFinalizer(ctx, namespace, endpointName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", constant.ErrUnknown)
				defer patches.Reset()

				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.RemoveFinalizer(ctx, namespace, endpointName)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("runs out of retries to update Endpoint, but conflicts still occur", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", apierrors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil))
				defer patches.Reset()

				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.RemoveFinalizer(ctx, namespace, endpointName)
				Expect(err).To(MatchError(constant.ErrRetriesExhausted))
			})

			It("removes the Endpoint's finalizer", func() {
				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.RemoveFinalizer(ctx, namespace, endpointName)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: endpointName}, &endpoint)
				Expect(err).NotTo(HaveOccurred())

				contains := controllerutil.ContainsFinalizer(&endpoint, constant.SpiderFinalizer)
				Expect(contains).To(BeFalse())
			})
		})

		Describe("MarkIPAllocation", func() {
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
				ctx := context.TODO()
				endpoint, err := endpointManager.MarkIPAllocation(ctx, stringid.GenerateRandomID(), nil)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
				Expect(endpoint).To(BeNil())
			})

			It("failed to get the top controller of Pod due to some unknown errors", func() {
				mockPodManager.EXPECT().
					GetPodTopController(gomock.All(), gomock.All()).
					Return(spiderpooltypes.PodTopController{}, constant.ErrUnknown).
					Times(1)

				ctx := context.TODO()
				endpoint, err := endpointManager.MarkIPAllocation(ctx, stringid.GenerateRandomID(), podT)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(endpoint).To(BeNil())
			})

			It("failed to set ownerReference to Pod due to some unknown errors", func() {
				mockPodManager.EXPECT().
					GetPodTopController(gomock.All(), gomock.All()).
					Return(spiderpooltypes.PodTopController{
						Kind:      constant.KindPod,
						Namespace: podT.Namespace,
						Name:      podT.Name,
						Uid:       podT.UID,
						App:       podT,
					}, nil).
					Times(1)

				patches := gomonkey.ApplyFuncReturn(controllerutil.SetOwnerReference, constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				endpoint, err := endpointManager.MarkIPAllocation(ctx, stringid.GenerateRandomID(), podT)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(endpoint).To(BeNil())
			})

			It("failed to create Endpoint due to some unknown errors", func() {
				mockPodManager.EXPECT().
					GetPodTopController(gomock.All(), gomock.All()).
					Return(spiderpooltypes.PodTopController{
						Kind:      constant.KindPod,
						Namespace: podT.Namespace,
						Name:      podT.Name,
						Uid:       podT.UID,
						App:       podT,
					}, nil).
					Times(1)

				patches := gomonkey.ApplyMethodReturn(fakeClient, "Create", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				endpoint, err := endpointManager.MarkIPAllocation(ctx, stringid.GenerateRandomID(), podT)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(endpoint).To(BeNil())
			})

			It("failed to update the status of Endpoint due to some unknown errors", func() {
				mockPodManager.EXPECT().
					GetPodTopController(gomock.All(), gomock.All()).
					Return(spiderpooltypes.PodTopController{
						Kind:      constant.KindPod,
						Namespace: podT.Namespace,
						Name:      podT.Name,
						Uid:       podT.UID,
						App:       podT,
					}, nil).
					Times(1)

				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				endpoint, err := endpointManager.MarkIPAllocation(ctx, stringid.GenerateRandomID(), podT)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(endpoint).To(BeNil())
			})

			It("marks the IP allocation for orphan Pod", func() {
				mockPodManager.EXPECT().
					GetPodTopController(gomock.All(), gomock.All()).
					Return(spiderpooltypes.PodTopController{
						Kind:      constant.KindPod,
						Namespace: podT.Namespace,
						Name:      podT.Name,
						Uid:       podT.UID,
						App:       podT,
					}, nil).
					Times(1)

				ctx := context.TODO()
				endpoint, err := endpointManager.MarkIPAllocation(ctx, stringid.GenerateRandomID(), podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint).NotTo(BeNil())
			})

			It("marks the IP allocation for StatefulSet's Pod", func() {
				mockPodManager.EXPECT().
					GetPodTopController(gomock.All(), gomock.All()).
					Return(spiderpooltypes.PodTopController{
						Kind:      constant.KindStatefulSet,
						Namespace: namespace,
						Name:      fmt.Sprintf("%s-sts", endpointName),
						Uid:       uuid.NewUUID(),
						App:       &appsv1.StatefulSet{},
					}, nil).
					Times(1)

				ctx := context.TODO()
				endpoint, err := endpointManager.MarkIPAllocation(ctx, stringid.GenerateRandomID(), podT)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint).NotTo(BeNil())
			})
		})

		Describe("ReMarkIPAllocation", func() {
			var containerID string
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

				containerID = stringid.GenerateRandomID()
				allocation := &spiderpoolv1.PodIPAllocation{
					ContainerID:  containerID,
					Node:         &podT.Spec.NodeName,
					CreationTime: &metav1.Time{Time: time.Now()},
				}

				err := controllerutil.SetOwnerReference(podT, endpointT, scheme)
				Expect(err).NotTo(HaveOccurred())

				controllerutil.AddFinalizer(endpointT, constant.SpiderFinalizer)
				endpointT.Status.Current = allocation
				endpointT.Status.History = append(endpointT.Status.History, *allocation)
			})

			It("inputs nil Pod", func() {
				ctx := context.TODO()
				err := endpointManager.ReMarkIPAllocation(ctx, stringid.GenerateRandomID(), nil, endpointT)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("inputs nil Endpoint", func() {
				ctx := context.TODO()
				err := endpointManager.ReMarkIPAllocation(ctx, stringid.GenerateRandomID(), podT, nil)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("test to create two Pods with the same namespace and name in a very short time", func() {
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Delete(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: endpointName}, &endpoint)
				Expect(err).NotTo(HaveOccurred())

				newPod := podT.DeepCopy()
				newPod.SetUID(uuid.NewUUID())
				err = endpointManager.ReMarkIPAllocation(ctx, stringid.GenerateRandomID(), newPod, &endpoint)
				Expect(err).To(HaveOccurred())
			})

			It("re-marks the IP allocation with the same container ID", func() {
				ctx := context.TODO()
				err := endpointManager.ReMarkIPAllocation(ctx, containerID, podT, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update the status of Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				ctx := context.TODO()
				err := endpointManager.ReMarkIPAllocation(ctx, stringid.GenerateRandomID(), podT, endpointT)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("re-marks the IP allocation", func() {
				By("Create the Endpoint")
				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				By("Inadvertently delete the Endpoint manually")
				err = fakeClient.Delete(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				By("Re-mark the Endpoint")
				var endpoint spiderpoolv1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: endpointName}, &endpoint)
				Expect(err).NotTo(HaveOccurred())

				newContainerID := stringid.GenerateRandomID()
				err = endpointManager.ReMarkIPAllocation(ctx, newContainerID, podT, &endpoint)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint.Status.Current.ContainerID).To(Equal(newContainerID))

				By("Truncate the extra history records")
				Expect(endpoint.Status.History).To(HaveLen(1))
				Expect(*endpoint.Status.Current).To(Equal(endpoint.Status.History[0]))
			})
		})

		Describe("PatchIPAllocation", func() {
			var marked *spiderpoolv1.PodIPAllocation
			var v4Patch, v6Patch *spiderpoolv1.PodIPAllocation

			BeforeEach(func() {
				containerID := stringid.GenerateRandomID()
				marked = &spiderpoolv1.PodIPAllocation{
					ContainerID:  containerID,
					Node:         pointer.String("node"),
					CreationTime: &metav1.Time{Time: time.Now()},
				}

				v4Patch = &spiderpoolv1.PodIPAllocation{
					ContainerID: containerID,
					IPs: []spiderpoolv1.IPAllocationDetail{
						{
							NIC:         "eth0",
							Vlan:        pointer.Int64(0),
							IPv4:        pointer.String("172.18.40.10/24"),
							IPv4Pool:    pointer.String("default-ipv4-ippool"),
							IPv4Gateway: pointer.String("172.18.40.1"),
							Routes: []spiderpoolv1.Route{
								{
									Dst: "192.168.40.0/24",
									Gw:  "172.18.40.1",
								},
							},
						},
					},
				}
				v6Patch = &spiderpoolv1.PodIPAllocation{
					ContainerID: containerID,
					IPs: []spiderpoolv1.IPAllocationDetail{
						{
							NIC:         "eth0",
							Vlan:        pointer.Int64(0),
							IPv6:        pointer.String("abcd:1234::a/120"),
							IPv6Pool:    pointer.String("default-ipv6-ippool"),
							IPv6Gateway: pointer.String("abcd:1234::1"),
							Routes: []spiderpoolv1.Route{
								{
									Dst: "fd00:40::/120",
									Gw:  "abcd:1234::1",
								},
							},
						},
					},
				}
			})

			It("inputs nil Endpoint", func() {
				ctx := context.TODO()
				err := endpointManager.PatchIPAllocation(ctx, v4Patch, nil)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("inputs nil IP allocation", func() {
				ctx := context.TODO()
				err := endpointManager.PatchIPAllocation(ctx, nil, endpointT)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("patches the IP allocation for unmarked Endpoint", func() {
				endpointT.Status.Current = nil

				ctx := context.TODO()
				err := endpointManager.PatchIPAllocation(ctx, v4Patch, endpointT)
				Expect(err).To(HaveOccurred())
			})

			It("patches the IP allocation when the Endpoint data is corrupt", func() {
				endpointT.Status.Current = marked

				ctx := context.TODO()
				err := endpointManager.PatchIPAllocation(ctx, v4Patch, endpointT)
				Expect(err).To(HaveOccurred())
			})

			It("patches the IP allocation with unmatched container ID", func() {
				endpointT.Status.Current = marked
				endpointT.Status.History = append(endpointT.Status.History, *marked)
				v4Patch.ContainerID = stringid.GenerateRandomID()

				ctx := context.TODO()
				err := endpointManager.PatchIPAllocation(ctx, v4Patch, endpointT)
				Expect(err).To(HaveOccurred())
			})

			It("failed to update the status of Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				endpointT.Status.Current = marked
				endpointT.Status.History = append(endpointT.Status.History, *marked)

				ctx := context.TODO()
				err := endpointManager.PatchIPAllocation(ctx, v4Patch, endpointT)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("patches the IP allocation of IPv4 and IPv6", func() {
				endpointT.Status.Current = marked
				endpointT.Status.History = append(endpointT.Status.History, *marked)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.PatchIPAllocation(ctx, v4Patch, endpointT)
				Expect(err).NotTo(HaveOccurred())
				err = endpointManager.PatchIPAllocation(ctx, v6Patch, endpointT)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: endpointName}, &endpoint)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint.Status.Current.IPs).To(HaveLen(1))
				Expect(endpoint.Status.Current.IPs[0]).To(Equal(
					spiderpoolv1.IPAllocationDetail{
						NIC:         "eth0",
						Vlan:        pointer.Int64(0),
						IPv4:        pointer.String("172.18.40.10/24"),
						IPv4Pool:    pointer.String("default-ipv4-ippool"),
						IPv4Gateway: pointer.String("172.18.40.1"),
						IPv6:        pointer.String("abcd:1234::a/120"),
						IPv6Pool:    pointer.String("default-ipv6-ippool"),
						IPv6Gateway: pointer.String("abcd:1234::1"),
						Routes: []spiderpoolv1.Route{
							{
								Dst: "192.168.40.0/24",
								Gw:  "172.18.40.1",
							},
							{
								Dst: "fd00:40::/120",
								Gw:  "abcd:1234::1",
							},
						},
					},
				))
				Expect(*endpoint.Status.Current).To(Equal(endpoint.Status.History[0]))
			})
		})

		Describe("ClearCurrentIPAllocation", func() {
			It("inputs nil Endpoint", func() {
				ctx := context.TODO()
				err := endpointManager.ClearCurrentIPAllocation(ctx, stringid.GenerateRandomID(), nil)
				Expect(err).NotTo(HaveOccurred())
			})

			It("clears up nil current IP allocation", func() {
				ctx := context.TODO()
				err := endpointManager.ClearCurrentIPAllocation(ctx, stringid.GenerateRandomID(), endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("clears up the current IP allocation with unmatched container ID", func() {
				endpointT.Status.Current.ContainerID = stringid.GenerateRandomID()

				ctx := context.TODO()
				err := endpointManager.ClearCurrentIPAllocation(ctx, stringid.GenerateRandomID(), endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update the status of Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				containerId := stringid.GenerateRandomID()
				endpointT.Status.Current.ContainerID = containerId

				ctx := context.TODO()
				err := endpointManager.ClearCurrentIPAllocation(ctx, containerId, endpointT)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("clears up the current IP allocation for non-existent Endpoint", func() {
				containerId := stringid.GenerateRandomID()
				endpointT.Status.Current.ContainerID = containerId

				ctx := context.TODO()
				err := endpointManager.ClearCurrentIPAllocation(ctx, containerId, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("clears up the current IP allocation", func() {
				containerId := stringid.GenerateRandomID()
				endpointT.Status.Current.ContainerID = containerId

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.ClearCurrentIPAllocation(ctx, containerId, endpointT)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: endpointName}, &endpoint)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint.Status.Current).To(BeNil())
			})
		})

		Describe("ReallocateCurrentIPAllocation", func() {
			It("reallocates the current IP allocation for non-existent Endpoint", func() {
				ctx := context.TODO()
				err := endpointManager.ReallocateCurrentIPAllocation(ctx, stringid.GenerateRandomID(), "node", namespace, endpointName)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
			})

			It("reallocates the current IP allocation with the same container ID", func() {
				containerID := stringid.GenerateRandomID()
				endpointT.Status.Current.ContainerID = containerID

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.ReallocateCurrentIPAllocation(ctx, containerID, "node", namespace, endpointName)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update the status of Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", constant.ErrUnknown)
				defer patches.Reset()

				endpointT.Status.Current.ContainerID = stringid.GenerateRandomID()
				endpointT.Status.Current.Node = pointer.String("old-node")

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.ReallocateCurrentIPAllocation(ctx, stringid.GenerateRandomID(), "new-node", namespace, endpointName)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("runs out of retries to update the status of Endpoint, but conflicts still occur", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient.Status(), "Update", apierrors.NewConflict(schema.GroupResource{Resource: "test"}, "other", nil))
				defer patches.Reset()

				endpointT.Status.Current.ContainerID = stringid.GenerateRandomID()
				endpointT.Status.Current.Node = pointer.String("old-node")

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				err = endpointManager.ReallocateCurrentIPAllocation(ctx, stringid.GenerateRandomID(), "new-node", namespace, endpointName)
				Expect(err).To(MatchError(constant.ErrRetriesExhausted))
			})

			It("updates the current IP allocation", func() {
				endpointT.Status.Current.ContainerID = stringid.GenerateRandomID()
				endpointT.Status.Current.Node = pointer.String("old-node")

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				containerID := stringid.GenerateRandomID()
				nodeName := "new-node"
				err = endpointManager.ReallocateCurrentIPAllocation(ctx, containerID, nodeName, namespace, endpointName)
				Expect(err).NotTo(HaveOccurred())

				var endpoint spiderpoolv1.SpiderEndpoint
				err = fakeClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: endpointName}, &endpoint)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpoint.Status.Current.ContainerID).To(Equal(containerID))
				Expect(*endpoint.Status.Current.Node).To(Equal(nodeName))
			})
		})
	})
})
