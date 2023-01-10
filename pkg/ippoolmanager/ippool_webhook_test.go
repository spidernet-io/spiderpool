// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var _ = Describe("IPPoolWebhook", Label("ippool_webhook_test"), func() {
	Describe("Set up SubnetWebhook", func() {
		PIt("talks to a Kubernetes API server", func() {
			cfg, err := config.GetConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).NotTo(BeNil())

			mgr, err := ctrl.NewManager(cfg, manager.Options{Scheme: scheme})
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolWebhook.SetupWebhookWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Test IPPoolWebhook's method", func() {
		var count uint64
		var subnetName string
		var subnetT *spiderpoolv1.SpiderSubnet
		var ipPoolName, existIPPoolName string
		var ipPoolT, existIPPoolT *spiderpoolv1.SpiderIPPool

		BeforeEach(func() {
			ippoolmanager.WebhookLogger = logutils.Logger.Named("IPPool-Webhook")
			ipPoolWebhook.EnableIPv4 = true
			ipPoolWebhook.EnableIPv6 = true
			ipPoolWebhook.EnableSpiderSubnet = false

			atomic.AddUint64(&count, 1)
			subnetName = fmt.Sprintf("subnet-%v", count)
			subnetT = &spiderpoolv1.SpiderSubnet{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderSubnetKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: subnetName,
				},
				Spec: spiderpoolv1.SubnetSpec{},
			}

			ipPoolName = fmt.Sprintf("ippool-%v", count)
			ipPoolT = &spiderpoolv1.SpiderIPPool{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderIPPoolKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: ipPoolName,
				},
				Spec: spiderpoolv1.IPPoolSpec{},
			}

			existIPPoolName = fmt.Sprintf("z-exist-ippool-%v", count)
			existIPPoolT = &spiderpoolv1.SpiderIPPool{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderIPPoolKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: existIPPoolName,
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

			err = fakeClient.Delete(ctx, existIPPoolT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = fakeClient.Delete(ctx, subnetT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("Default", func() {
			It("avoids modifying the terminating IPPool", func() {
				now := metav1.Now()
				ipPoolT.SetDeletionTimestamp(&now)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to list Subnets due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				subnetT.Spec.Subnet = "172.18.40.0/24"
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.40.1-172.18.40.2",
						"172.18.40.10",
					}...,
				)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"

				err = ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				controlled := metav1.IsControlledBy(ipPoolT, subnetT)
				Expect(controlled).NotTo(BeTrue())

				v, ok := ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
				Expect(ok).NotTo(BeTrue())
				Expect(v).NotTo(Equal(subnetName))
			})

			It("failed to set owner reference due to some unknown errors", func() {
				patches := gomonkey.ApplyFuncReturn(controllerutil.SetControllerReference, constant.ErrUnknown)
				defer patches.Reset()

				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				subnetT.Spec.Subnet = "172.18.40.0/24"
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.40.1-172.18.40.2",
						"172.18.40.10",
					}...,
				)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"

				err = ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				controlled := metav1.IsControlledBy(ipPoolT, subnetT)
				Expect(controlled).NotTo(BeTrue())

				v, ok := ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
				Expect(ok).NotTo(BeTrue())
				Expect(v).NotTo(Equal(subnetName))
			})

			It("sets the reference of the controller Subnet", func() {
				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				subnetT.Spec.Subnet = "172.18.40.0/24"
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.40.1-172.18.40.2",
						"172.18.40.10",
					}...,
				)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"

				err = ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				controlled := metav1.IsControlledBy(ipPoolT, subnetT)
				Expect(controlled).To(BeTrue())

				v, ok := ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(subnetName))

				err = ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds finalizer", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				contains := controllerutil.ContainsFinalizer(ipPoolT, constant.SpiderFinalizer)
				Expect(contains).To(BeTrue())
			})

			It("failed to set 'spec.ipVersion' due to the invalid 'spec.subnet'", func() {
				ipPoolT.Spec.Subnet = constant.InvalidCIDR

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.IPVersion).To(BeNil())
			})

			It("sets 'spec.ipVersion' to 4", func() {
				ipPoolT.Spec.Subnet = "172.18.40.0/24"

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(*ipPoolT.Spec.IPVersion).To(Equal(constant.IPv4))
			})

			It("sets 'spec.ipVersion' to 6", func() {
				ipPoolT.Spec.Subnet = "abcd:1234::/120"

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(*ipPoolT.Spec.IPVersion).To(Equal(constant.IPv6))
			})

			It("failed to merge 'spec.ips' due to the invalid 'spec.ipVersion'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.InvalidIPVersion)
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.IPs).To(Equal(
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					},
				))
			})

			It("failed to merge 'spec.ips' due to the invalid 'spec.ips'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						constant.InvalidIPRange,
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.IPs).To(Equal(
					[]string{
						constant.InvalidIPRange,
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					},
				))
			})

			It("merges IPv4 'spec.ips'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.IPs).To(Equal(
					[]string{
						"172.18.40.1-172.18.40.3",
						"172.18.40.10",
					},
				))
			})

			It("merges IPv6 'spec.ips'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"abcd:1234::a",
						"abcd:1234::1-abcd:1234::2",
						"abcd:1234::2-abcd:1234::3",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.IPs).To(Equal(
					[]string{
						"abcd:1234::1-abcd:1234::3",
						"abcd:1234::a",
					},
				))
			})

			It("failed to merge 'spec.excludeIPs' due to the invalid 'spec.ipVersion'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.InvalidIPVersion)
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.ExcludeIPs).To(Equal(
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					},
				))
			})

			It("failed to merge 'spec.excludeIPs' due to the invalid 'spec.ips'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						constant.InvalidIPRange,
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.ExcludeIPs).To(Equal(
					[]string{
						constant.InvalidIPRange,
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					},
				))
			})

			It("merges IPv4 'spec.excludeIPs'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.ExcludeIPs).To(Equal(
					[]string{
						"172.18.40.1-172.18.40.3",
						"172.18.40.10",
					},
				))
			})

			It("merges IPv6 'spec.excludeIPs'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						"abcd:1234::a",
						"abcd:1234::1-abcd:1234::2",
						"abcd:1234::2-abcd:1234::3",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.ExcludeIPs).To(Equal(
					[]string{
						"abcd:1234::1-abcd:1234::3",
						"abcd:1234::a",
					},
				))
			})
		})

		Describe("ValidateCreate", func() {
			When("Validating 'spec.ipVersion'", func() {
				It("inputs nil 'spec.ipVersion'", func() {
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs invalid 'spec.ipVersion'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.InvalidIPVersion)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("creates IPv4 IPPool but IPv4 is disbale'", func() {
					ipPoolWebhook.EnableIPv4 = false
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("creates IPv6 IPPool but IPv6 is disbale'", func() {
					ipPoolWebhook.EnableIPv6 = false
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)
					ipPoolT.Spec.Subnet = "adbc:1234::/120"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"adbc:1234::1-adbc:1234::2",
							"adbc:1234::a",
						}...,
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.subnet'", func() {
				It("inputs invalid 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = constant.InvalidCIDR
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("failed to list IPPools due to some unknown errors", func() {
					patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
					defer patches.Reset()

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("creates an existing IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("creates an IPPool with the same 'spec.subnet'", func() {
					existIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					existIPPoolT.Spec.Subnet = "172.18.40.0/24"
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, "172.18.41.1-172.18.41.2")

					ctx := context.TODO()
					err := fakeClient.Create(ctx, existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.10")

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

				It("failed to compare 'spec.subnet' with existing Subnet", func() {
					existIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					existIPPoolT.Spec.Subnet = constant.InvalidCIDR
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, "172.18.41.1-172.18.41.2")

					ctx := context.TODO()
					err := fakeClient.Create(ctx, existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("overlaps with existing Subnet", func() {
					existIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					existIPPoolT.Spec.Subnet = "172.18.40.0/25"
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, "172.18.40.40")

					ctx := context.TODO()
					err := fakeClient.Create(ctx, existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.ips'", func() {
				It("inputs empty 'spec.ips' while feature SpiderSubnet is enabled", func() {
					ipPoolWebhook.EnableSpiderSubnet = true
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

				It("inputs empty 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs invalid 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs 'spec.ips' that do not pertains to 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.41.10",
						}...,
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				PIt("failed to list IPPools due to some unknown errors", func() {})

				PIt("failed to assemble the total IP addresses of the IPPool due to some unknown errors", func() {})

				It("exists invalid IPPool in the cluster", func() {
					existIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					existIPPoolT.Spec.Subnet = "172.18.40.0/24"
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("overlaps with existing IPPool", func() {
					existIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					existIPPoolT.Spec.Subnet = "172.18.40.0/24"
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := fakeClient.Create(ctx, existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.excludeIPs'", func() {
				It("inputs invalid 'spec.excludeIPs'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs 'spec.excludeIPs' that do not pertains to 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs, "172.18.41.10")

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.gateway'", func() {
				It("inputs invalid 'spec.gateway'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Gateway = pointer.String(constant.InvalidIP)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs 'spec.gateway' that do not pertains to 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Gateway = pointer.String("172.18.41.1")

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.routes'", func() {
				It("inputs invalid destination", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Routes = append(ipPoolT.Spec.Routes,
						spiderpoolv1.Route{
							Dst: constant.InvalidCIDR,
							Gw:  "172.18.40.1",
						},
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs invalid gateway", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Routes = append(ipPoolT.Spec.Routes,
						spiderpoolv1.Route{
							Dst: "192.168.40.0/24",
							Gw:  constant.InvalidIP,
						},
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs gateway that do not pertains to 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Routes = append(ipPoolT.Spec.Routes,
						spiderpoolv1.Route{
							Dst: "192.168.40.0/24",
							Gw:  "172.18.41.1",
						},
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating the existence of the controller Subnet", func() {
				BeforeEach(func() {
					ipPoolWebhook.EnableSpiderSubnet = true
				})

				It("is orphan IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("sets owner reference to non-existent Subnet", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err := controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("sets owner reference to a Subnet with different 'spec.subnet'", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/25"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("sets owner reference to a terminating Subnet", func() {
					controllerutil.AddFinalizer(subnetT, constant.SpiderFinalizer)
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = fakeClient.Delete(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating the total IP addresses contained in the controller Subnet", func() {
				BeforeEach(func() {
					ipPoolWebhook.EnableSpiderSubnet = true
				})

				PIt("failed to assemble the total IP addresses of the IPPool due to some unknown errors", func() {})

				PIt("failed to assemble the total IP addresses of the Subnet due to some unknown errors", func() {})

				It("is out of the IP range of the Subnet", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.40",
						}...,
					)

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			It("creates IPv4 Subnet with all fields valid", func() {
				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				subnetT.Spec.Subnet = "172.18.40.0/24"
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.40.2-172.18.40.3",
						"172.18.40.10",
					}...,
				)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.18.40.2-172.18.40.3",
						"172.18.40.10",
					}...,
				)
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs, "172.18.40.10")
				ipPoolT.Spec.Gateway = pointer.String("172.18.40.1")
				ipPoolT.Spec.Vlan = pointer.Int64(0)
				ipPoolT.Spec.Routes = append(ipPoolT.Spec.Routes,
					spiderpoolv1.Route{
						Dst: "192.168.40.0/24",
						Gw:  "172.18.40.40",
					},
				)

				err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("creates IPv6 Subnet with all fields valid", func() {
				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Spec.IPVersion = pointer.Int64(constant.IPv6)
				subnetT.Spec.Subnet = "abcd:1234::/120"
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"abcd:1234::2-abcd:1234::3",
						"abcd:1234::a",
					}...,
				)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)
				ipPoolT.Spec.Subnet = "abcd:1234::/120"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"abcd:1234::2-abcd:1234::3",
						"abcd:1234::a",
					}...,
				)
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs, "abcd:1234::a")
				ipPoolT.Spec.Gateway = pointer.String("abcd:1234::1")
				ipPoolT.Spec.Vlan = pointer.Int64(0)
				ipPoolT.Spec.Routes = append(ipPoolT.Spec.Routes,
					spiderpoolv1.Route{
						Dst: "fd00:40::/120",
						Gw:  "abcd:1234::28",
					},
				)

				err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("ValidateUpdate", func() {
			When("Validating 'spec.ipVersion'", func() {
				It("updates 'spec.ipVersion' to nil", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPVersion = nil

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("changes 'spec.ipVersion'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates IPv4 IPPool but IPv4 is disbale'", func() {
					ipPoolWebhook.EnableIPv4 = false
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates IPv6 IPPool but IPv6 is disbale'", func() {
					ipPoolWebhook.EnableIPv6 = false
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)
					ipPoolT.Spec.Subnet = "adbc:1234::/120"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "adbc:1234::1-adbc:1234::2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "adbc:1234::a")

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.subnet'", func() {
				It("changes 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Subnet = "172.18.40.0/25"

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.ips'", func() {
				It("removes all 'spec.ips' while feature SpiderSubnet is enabled", func() {
					ipPoolWebhook.EnableSpiderSubnet = true
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = nil

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

				It("removes all 'spec.ips'", func() {
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = nil

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("appends invalid IP range to 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("appends IP range that do not pertains to 'spec.subnet' to 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.41.10")

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				PIt("failed to list IPPools due to some unknown errors", func() {})

				PIt("failed to assemble the total IP addresses of the IPPool due to some unknown errors", func() {})

				It("exists invalid IPPool in the cluster", func() {
					existIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					existIPPoolT.Spec.Subnet = "172.18.40.0/24"
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("overlaps with existing IPPool", func() {
					existIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					existIPPoolT.Spec.Subnet = "172.18.40.0/24"
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := fakeClient.Create(ctx, existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.excludeIPs'", func() {
				It("appends invalid IP range to 'spec.excludeIPs'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.ExcludeIPs = append(newIPPoolT.Spec.ExcludeIPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("appends IP range that do not pertains to 'spec.subnet' to 'spec.excludeIPs'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.ExcludeIPs = append(newIPPoolT.Spec.ExcludeIPs, "172.18.41.10")

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.gateway'", func() {
				It("updates 'spec.gateway' to invalid gateway", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Gateway = pointer.String("172.18.40.1")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Gateway = pointer.String(constant.InvalidIP)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates 'spec.gateway' to a gateway that do not pertains to 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Gateway = pointer.String("172.18.40.1")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Gateway = pointer.String("172.18.41.1")

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.routes'", func() {
				It("appends route with invalid destination", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Routes = append(newIPPoolT.Spec.Routes,
						spiderpoolv1.Route{
							Dst: constant.InvalidCIDR,
							Gw:  "172.18.40.1",
						},
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("appends route with invalid gateway", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Routes = append(newIPPoolT.Spec.Routes,
						spiderpoolv1.Route{
							Dst: "192.168.40.0/24",
							Gw:  constant.InvalidIP,
						},
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("appends route whose gateway does not pertains to 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Routes = append(newIPPoolT.Spec.Routes,
						spiderpoolv1.Route{
							Dst: "192.168.40.0/24",
							Gw:  "172.18.41.1",
						},
					)

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating the IP addresses being used", func() {
				PIt("failed to assemble the total IP addresses of the IPPool due to some unknown errors", func() {})

				It("removes IP range that is being used by IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ipPoolT.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{
						"172.18.40.10": spiderpoolv1.PoolIPAllocation{},
					}

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = newIPPoolT.Spec.IPs[:1]

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating the existence of the controller Subnet", func() {
				BeforeEach(func() {
					ipPoolWebhook.EnableSpiderSubnet = true
				})

				It("updates orphan IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates the IPPool that sets the owner reference to non-existent Subnet", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err := controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates the IPPool that sets the owner reference to a Subnet with different 'spec.subnet'", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/25"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates the IPPool that sets the owner reference to a terminating Subnet", func() {
					controllerutil.AddFinalizer(subnetT, constant.SpiderFinalizer)
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = fakeClient.Delete(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating the total IP addresses contained in the controller Subnet", func() {
				BeforeEach(func() {
					ipPoolWebhook.EnableSpiderSubnet = true
				})

				PIt("failed to assemble the total IP addresses of the IPPool due to some unknown errors", func() {})

				PIt("failed to assemble the total IP addresses of the Subnet due to some unknown errors", func() {})

				It("is out of the IP range of the Subnet", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.1-172.18.40.2")

					ctx := context.TODO()
					err := fakeClient.Create(ctx, subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			It("deletes IPPool", func() {
				controllerutil.AddFinalizer(ipPoolT, constant.SpiderFinalizer)
				now := metav1.Now()
				ipPoolT.SetDeletionTimestamp(&now)
				ipPoolT.SetDeletionGracePeriodSeconds(pointer.Int64(0))

				newIPPoolT := ipPoolT.DeepCopy()
				controllerutil.RemoveFinalizer(newIPPoolT, constant.SpiderFinalizer)

				ctx := context.TODO()
				err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates terminating Subnet", func() {
				controllerutil.AddFinalizer(ipPoolT, constant.SpiderFinalizer)
				now := metav1.Now()
				ipPoolT.SetDeletionTimestamp(&now)
				ipPoolT.SetDeletionGracePeriodSeconds(pointer.Int64(30))

				newIPPoolT := ipPoolT.DeepCopy()

				ctx := context.TODO()
				err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("updates IPv4 Subnet with all fields valid", func() {
				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				subnetT.Spec.Subnet = "172.18.40.0/24"
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.40.2-172.18.40.3",
						"172.18.40.10",
					}...,
				)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.2-172.18.40.3")
				ipPoolT.Spec.Vlan = pointer.Int64(0)

				newIPPoolT := ipPoolT.DeepCopy()
				newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")
				newIPPoolT.Spec.ExcludeIPs = append(newIPPoolT.Spec.ExcludeIPs, "172.18.40.10")
				newIPPoolT.Spec.Gateway = pointer.String("172.18.40.1")
				newIPPoolT.Spec.Routes = append(newIPPoolT.Spec.Routes,
					spiderpoolv1.Route{
						Dst: "192.168.40.0/24",
						Gw:  "172.18.40.40",
					},
				)

				err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates IPv6 Subnet with all fields valid", func() {
				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Spec.IPVersion = pointer.Int64(constant.IPv6)
				subnetT.Spec.Subnet = "abcd:1234::/120"
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"abcd:1234::2-abcd:1234::3",
						"abcd:1234::a",
					}...,
				)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)
				ipPoolT.Spec.Subnet = "abcd:1234::/120"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "abcd:1234::2-abcd:1234::3")
				ipPoolT.Spec.Vlan = pointer.Int64(0)

				newIPPoolT := ipPoolT.DeepCopy()
				newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "abcd:1234::a")
				newIPPoolT.Spec.ExcludeIPs = append(newIPPoolT.Spec.ExcludeIPs, "abcd:1234::a")
				newIPPoolT.Spec.Gateway = pointer.String("abcd:1234::1")
				newIPPoolT.Spec.Routes = append(newIPPoolT.Spec.Routes,
					spiderpoolv1.Route{
						Dst: "fd00:40::/120",
						Gw:  "abcd:1234::28",
					},
				)

				err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("ValidateDelete", func() {
			It("passes", func() {
				ctx := context.TODO()
				err := ipPoolWebhook.ValidateDelete(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
