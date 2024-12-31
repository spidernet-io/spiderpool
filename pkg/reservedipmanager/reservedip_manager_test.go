// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
)

var _ = Describe("ReservedIPManager", Label("reservedip_manager_test"), func() {
	Describe("New ReservedIPManager", func() {
		It("inputs nil client", func() {
			manager, err := reservedipmanager.NewReservedIPManager(nil, fakeAPIReader)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})

		It("inputs nil API reader", func() {
			manager, err := reservedipmanager.NewReservedIPManager(fakeClient, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(manager).To(BeNil())
		})
	})

	Describe("Test ReservedIPManager's method", func() {
		var ctx context.Context

		var count uint64
		var rIPName string
		var labels map[string]string
		var rIPT, terminatingV4RIPT *spiderpoolv2beta1.SpiderReservedIP

		BeforeEach(func() {
			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			rIPName = fmt.Sprintf("reservedip-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			rIPT = &spiderpoolv2beta1.SpiderReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderReservedIP,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   rIPName,
					Labels: labels,
				},
				Spec: spiderpoolv2beta1.ReservedIPSpec{},
			}

			terminatingV4RIPT = &spiderpoolv2beta1.SpiderReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderReservedIP,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       "terminating-ipv4-reservedip",
					DeletionGracePeriodSeconds: ptr.To(int64(30)),
					Finalizers:                 []string{constant.SpiderFinalizer},
				},
				Spec: spiderpoolv2beta1.ReservedIPSpec{
					IPVersion: ptr.To(constant.IPv4),
					IPs: []string{
						"172.18.40.40",
					},
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

			err := fakeClient.Delete(ctx, rIPT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = fakeClient.Delete(ctx, terminatingV4RIPT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    constant.SpiderpoolAPIGroup,
					Version:  constant.SpiderpoolAPIVersion,
					Resource: "spiderreservedips",
				},
				rIPT.Namespace,
				rIPT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    constant.SpiderpoolAPIGroup,
					Version:  constant.SpiderpoolAPIVersion,
					Resource: "spiderendpoints",
				},
				terminatingV4RIPT.Namespace,
				terminatingV4RIPT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("GetReservedIPByName", func() {
			It("gets non-existent ReservedIP", func() {
				rIP, err := rIPManager.GetReservedIPByName(ctx, rIPName, constant.IgnoreCache)
				Expect(apierrors.IsNotFound(err)).To(BeTrue())
				Expect(rIP).To(BeNil())
			})

			It("gets an existing ReservedIP through cache", func() {
				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIP, err := rIPManager.GetReservedIPByName(ctx, rIPName, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIP).NotTo(BeNil())
				Expect(rIP).To(Equal(rIPT))
			})

			It("gets an existing ReservedIP through API Server", func() {
				err := tracker.Add(rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIP, err := rIPManager.GetReservedIPByName(ctx, rIPName, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIP).NotTo(BeNil())
				Expect(rIP).To(Equal(rIPT))
			})
		})

		Describe("ListReservedIPs", func() {
			It("failed to list ReservedIPs due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeAPIReader, "List", constant.ErrUnknown)
				defer patches.Reset()

				err := tracker.Add(rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx, constant.IgnoreCache)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(rIPList).To(BeNil())
			})

			It("lists all ReservedIPs through cache", func() {
				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx, constant.UseCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPList.Items).NotTo(BeEmpty())

				hasRIP := false
				for _, rIP := range rIPList.Items {
					if rIP.Name == rIPName {
						hasRIP = true
						break
					}
				}
				Expect(hasRIP).To(BeTrue())
			})

			It("lists all ReservedIPs through API Server", func() {
				err := tracker.Add(rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx, constant.IgnoreCache)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPList.Items).NotTo(BeEmpty())

				hasRIP := false
				for _, rIP := range rIPList.Items {
					if rIP.Name == rIPName {
						hasRIP = true
						break
					}
				}
				Expect(hasRIP).To(BeTrue())
			})

			It("filters results by label selector", func() {
				err := tracker.Add(rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx, constant.IgnoreCache, client.MatchingLabels(labels))
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPList.Items).NotTo(BeEmpty())

				hasRIP := false
				for _, rIP := range rIPList.Items {
					if rIP.Name == rIPName {
						hasRIP = true
						break
					}
				}
				Expect(hasRIP).To(BeTrue())
			})

			It("filters results by field selector", func() {
				err := tracker.Add(rIPT)
				Expect(err).NotTo(HaveOccurred())

				rIPList, err := rIPManager.ListReservedIPs(ctx, constant.IgnoreCache, client.MatchingFields{metav1.ObjectNameField: rIPName})
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPList.Items).NotTo(BeEmpty())

				hasRIP := false
				for _, rIP := range rIPList.Items {
					if rIP.Name == rIPName {
						hasRIP = true
						break
					}
				}
				Expect(hasRIP).To(BeTrue())
			})
		})

		Describe("AssembleReservedIPs", func() {
			It("inputs invalid IP version", func() {
				ips, err := rIPManager.AssembleReservedIPs(ctx, constant.InvalidIPVersion)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
				Expect(ips).To(BeEmpty())
			})

			It("failed to list ReservedIPs due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ips, err := rIPManager.AssembleReservedIPs(ctx, constant.IPv4)
				Expect(err).To(MatchError(constant.ErrUnknown))
				Expect(ips).To(BeEmpty())
			})

			It("does not assemble terminating IPv4 reserved-IP addresses", func() {
				rIPT.Spec.IPVersion = ptr.To(constant.IPv4)
				rIPT.Spec.IPs = []string{
					"172.18.40.1-172.18.40.2",
					"172.18.40.10",
				}

				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, terminatingV4RIPT)
				Expect(err).NotTo(HaveOccurred())

				// set it to terminating
				err = fakeClient.Delete(ctx, terminatingV4RIPT)
				Expect(err).NotTo(HaveOccurred())

				ips, err := rIPManager.AssembleReservedIPs(ctx, constant.IPv4)
				Expect(err).NotTo(HaveOccurred())
				Expect(ips).To(Equal(
					[]net.IP{
						net.IPv4(172, 18, 40, 1),
						net.IPv4(172, 18, 40, 2),
						net.IPv4(172, 18, 40, 10),
					},
				))
			})

			It("exists invalid ReservedIPs in the cluster", func() {
				rIPT.Spec.IPVersion = ptr.To(constant.IPv4)
				rIPT.Spec.IPs = append(rIPT.Spec.IPs, constant.InvalidIPRange)

				err := fakeClient.Create(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())

				ips, err := rIPManager.AssembleReservedIPs(ctx, constant.IPv4)
				Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
				Expect(ips).To(BeEmpty())
			})
		})
	})
})
