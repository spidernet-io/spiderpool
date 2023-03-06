// Copyright 2019 The Kubernetes Authors
// SPDX-License-Identifier: Apache-2.0

package workloadendpointmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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
	"github.com/spidernet-io/spiderpool/pkg/workloadendpointmanager"
)

var _ = Describe("WorkloadEndpointManager", Label("workloadendpoint_manager_test"), func() {
	Describe("New WorkloadEndpointManager", func() {
		It("sets default config", func() {
			manager, err := workloadendpointmanager.NewWorkloadEndpointManager(
				workloadendpointmanager.EndpointManagerConfig{},
				fakeClient,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(manager).NotTo(BeNil())
		})

		It("inputs nil client", func() {
			manager, err := workloadendpointmanager.NewWorkloadEndpointManager(
				workloadendpointmanager.EndpointManagerConfig{},
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
					Kind:       constant.KindSpiderEndpoint,
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

		PDescribe("PatchIPAllocationResults", func() {
		})

		Describe("ReallocateCurrentIPAllocation", func() {
			It("inputs nil Endpoint", func() {
				ctx := context.TODO()
				err := endpointManager.ReallocateCurrentIPAllocation(ctx, "", string(uuid.NewUUID()), "node1", nil)
				Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			})

			It("re-allocates but the Endpoint data is broken", func() {
				endpointT.Status.Current = nil

				ctx := context.TODO()
				err := endpointManager.ReallocateCurrentIPAllocation(ctx, "", string(uuid.NewUUID()), "node1", endpointT)
				Expect(err).To(HaveOccurred())
			})

			It("re-allocates the current IP allocation with the same container ID, Pod UID and Node name", func() {
				uid := string(uuid.NewUUID())
				nodeName := "node1"

				endpointT.Status.Current.UID = uid
				endpointT.Status.Current.Node = nodeName

				ctx := context.TODO()
				err := endpointManager.ReallocateCurrentIPAllocation(ctx, "", uid, nodeName, endpointT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to update the status of Endpoint due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "Update", constant.ErrUnknown)
				defer patches.Reset()

				endpointT.Status.Current.UID = string(uuid.NewUUID())
				endpointT.Status.Current.Node = "node1"

				ctx := context.TODO()
				err := endpointManager.ReallocateCurrentIPAllocation(ctx, "", string(uuid.NewUUID()), "node1", endpointT)
				Expect(err).To(MatchError(constant.ErrUnknown))
			})

			It("updates the current IP allocation", func() {
				endpointT.Status.Current.UID = string(uuid.NewUUID())
				endpointT.Status.Current.Node = "old-node"

				ctx := context.TODO()
				err := fakeClient.Create(ctx, endpointT)
				Expect(err).NotTo(HaveOccurred())

				uid := string(uuid.NewUUID())
				nodeName := "new-node"

				err = endpointManager.ReallocateCurrentIPAllocation(ctx, "", uid, nodeName, endpointT)
				Expect(err).NotTo(HaveOccurred())
				Expect(endpointT.Status.Current.UID).To(Equal(uid))
				Expect(endpointT.Status.Current.Node).To(Equal(nodeName))
			})
		})
	})
})
