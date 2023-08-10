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
	appsv1 "k8s.io/api/apps/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/types"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
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
		var ctx context.Context

		var count uint64
		var subnetName string
		var subnetT *spiderpoolv2beta1.SpiderSubnet
		var ipPoolName, existIPPoolName string
		var ipPoolT, existIPPoolT *spiderpoolv2beta1.SpiderIPPool

		BeforeEach(func() {
			ippoolmanager.WebhookLogger = logutils.Logger.Named("IPPool-Webhook")
			ipPoolWebhook.EnableIPv4 = true
			ipPoolWebhook.EnableIPv6 = true
			ipPoolWebhook.EnableSpiderSubnet = false

			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			subnetName = fmt.Sprintf("subnet-%v", count)
			subnetT = &spiderpoolv2beta1.SpiderSubnet{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderSubnet,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   subnetName,
					Labels: map[string]string{},
				},
				Spec: spiderpoolv2beta1.SubnetSpec{},
			}

			ipPoolName = fmt.Sprintf("ippool-%v", count)
			ipPoolT = &spiderpoolv2beta1.SpiderIPPool{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderIPPool,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: ipPoolName,
				},
				Spec: spiderpoolv2beta1.IPPoolSpec{},
			}

			existIPPoolName = fmt.Sprintf("z-exist-ippool-%v", count)
			existIPPoolT = &spiderpoolv2beta1.SpiderIPPool{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderIPPool,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   existIPPoolName,
					Labels: map[string]string{},
				},
				Spec: spiderpoolv2beta1.IPPoolSpec{},
			}
		})

		var deleteOption *client.DeleteOptions

		AfterEach(func() {
			policy := metav1.DeletePropagationForeground
			deleteOption = &client.DeleteOptions{
				GracePeriodSeconds: pointer.Int64(0),
				PropagationPolicy:  &policy,
			}

			err := fakeClient.Delete(ctx, ipPoolT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = fakeClient.Delete(ctx, existIPPoolT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = fakeClient.Delete(ctx, subnetT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    constant.SpiderpoolAPIGroup,
					Version:  constant.SpiderpoolAPIVersion,
					Resource: "spiderippools",
				},
				ipPoolT.Namespace,
				ipPoolT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    constant.SpiderpoolAPIGroup,
					Version:  constant.SpiderpoolAPIVersion,
					Resource: "spiderippools",
				},
				existIPPoolT.Namespace,
				existIPPoolT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())

			err = tracker.Delete(
				schema.GroupVersionResource{
					Group:    constant.SpiderpoolAPIGroup,
					Version:  constant.SpiderpoolAPIVersion,
					Resource: "spidersubnets",
				},
				subnetT.Namespace,
				subnetT.Name,
			)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("Default", func() {
			It("avoids modifying the terminating IPPool", func() {
				now := metav1.Now()
				ipPoolT.SetDeletionTimestamp(&now)

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("adds finalizer", func() {
				ipPoolT.Spec.Subnet = "172.18.40.0/24"

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				contains := controllerutil.ContainsFinalizer(ipPoolT, constant.SpiderFinalizer)
				Expect(contains).To(BeTrue())
			})

			It("failed to parse 'spec.subnet' as a valid label value", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.InvalidIPVersion)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				v, ok := ipPoolT.Labels[constant.LabelIPPoolCIDR]
				Expect(ok).To(BeFalse())
				Expect(v).To(BeEmpty())
			})

			It("sets CIDR label", func() {
				subnet := "172.18.40.0/24"
				ipPoolT.Spec.Subnet = subnet

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				cidr, err := spiderpoolip.CIDRToLabelValue(*ipPoolT.Spec.IPVersion, subnet)
				Expect(err).NotTo(HaveOccurred())
				Expect(cidr).NotTo(BeEmpty())

				v, ok := ipPoolT.Labels[constant.LabelIPPoolCIDR]
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(cidr))
			})

			It("is orphan IPPool", func() {
				ipPoolWebhook.EnableSpiderSubnet = true
				ipPoolT.Spec.Subnet = "172.18.40.0/24"

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				owner := metav1.GetControllerOf(ipPoolT)
				Expect(owner).To(BeNil())
			})

			It("failed to list Subnets due to some unknown errors", func() {
				patches := gomonkey.ApplyMethodReturn(fakeClient, "List", constant.ErrUnknown)
				defer patches.Reset()

				ipVersion := constant.IPv4
				subnet := "172.18.40.0/24"
				cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
				Expect(err).NotTo(HaveOccurred())
				Expect(cidr).NotTo(BeEmpty())

				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Labels[constant.LabelSubnetCIDR] = cidr
				subnetT.Spec.IPVersion = pointer.Int64(ipVersion)
				subnetT.Spec.Subnet = subnet
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.40.1-172.18.40.2",
						"172.18.40.10",
					}...,
				)

				err = fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.Subnet = subnet
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

				ipVersion := constant.IPv4
				subnet := "172.18.40.0/24"
				cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
				Expect(err).NotTo(HaveOccurred())
				Expect(cidr).NotTo(BeEmpty())

				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Labels[constant.LabelSubnetCIDR] = cidr
				subnetT.Spec.IPVersion = pointer.Int64(ipVersion)
				subnetT.Spec.Subnet = subnet
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.40.1-172.18.40.2",
						"172.18.40.10",
					}...,
				)

				err = fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.Subnet = subnet
				err = ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				controlled := metav1.IsControlledBy(ipPoolT, subnetT)
				Expect(controlled).NotTo(BeTrue())

				v, ok := ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
				Expect(ok).NotTo(BeTrue())
				Expect(v).NotTo(Equal(subnetName))
			})

			It("sets the reference of the controller Subnet", func() {
				ipVersion := constant.IPv4
				subnet := "172.18.50.0/24"
				cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
				Expect(err).NotTo(HaveOccurred())
				Expect(cidr).NotTo(BeEmpty())

				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Labels[constant.LabelSubnetCIDR] = cidr
				subnetT.Spec.IPVersion = pointer.Int64(ipVersion)
				subnetT.Spec.Subnet = subnet
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.50.1-172.18.50.2",
						"172.18.50.10",
					}...,
				)

				err = fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.Subnet = subnet
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

			It("failed to set 'spec.ipVersion' due to the invalid 'spec.subnet'", func() {
				ipPoolT.Spec.Subnet = constant.InvalidCIDR

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.IPVersion).To(BeNil())
			})

			It("sets 'spec.ipVersion' to 4", func() {
				ipPoolT.Spec.Subnet = "172.18.40.0/24"

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(*ipPoolT.Spec.IPVersion).To(Equal(constant.IPv4))
			})

			It("sets 'spec.ipVersion' to 6", func() {
				ipPoolT.Spec.Subnet = "abcd:1234::/120"

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(*ipPoolT.Spec.IPVersion).To(Equal(constant.IPv6))
			})

			It("failed to merge 'spec.ips' due to the invalid 'spec.ipVersion'", func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.InvalidIPVersion)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

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
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						constant.InvalidIPRange,
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

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
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

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
				ipPoolT.Spec.Subnet = "abcd:1234::/120"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"abcd:1234::a",
						"abcd:1234::1-abcd:1234::2",
						"abcd:1234::2-abcd:1234::3",
					}...,
				)

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
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

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
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						constant.InvalidIPRange,
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

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
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

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
				ipPoolT.Spec.Subnet = "abcd:1234::/120"
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						"abcd:1234::a",
						"abcd:1234::1-abcd:1234::2",
						"abcd:1234::2-abcd:1234::3",
					}...,
				)

				err := ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
				Expect(ipPoolT.Spec.ExcludeIPs).To(Equal(
					[]string{
						"abcd:1234::1-abcd:1234::3",
						"abcd:1234::a",
					},
				))
			})

			It("inherit subnet properties from SpiderSubnet", func() {
				ipVersion := constant.IPv4
				subnet := "172.18.50.0/24"
				cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
				Expect(err).NotTo(HaveOccurred())
				Expect(cidr).NotTo(BeEmpty())

				ipPoolWebhook.EnableSpiderSubnet = true
				subnetT.SetUID(uuid.NewUUID())
				subnetT.Labels[constant.LabelSubnetCIDR] = cidr
				subnetT.Spec.IPVersion = pointer.Int64(ipVersion)
				subnetT.Spec.Subnet = subnet
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.18.50.1-172.18.50.2",
						"172.18.50.10",
					}...,
				)

				subnetT.Spec.Gateway = pointer.String("172.18.50.0")
				subnetT.Spec.Vlan = pointer.Int64(50)
				subnetT.Spec.Routes = []spiderpoolv2beta1.Route{
					{
						Dst: "0.0.0.0/0",
						Gw:  "172.18.50.0",
					},
				}

				err = fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				ipPoolT.Spec.Subnet = subnet
				err = ipPoolWebhook.Default(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				controlled := metav1.IsControlledBy(ipPoolT, subnetT)
				Expect(controlled).To(BeTrue())

				v, ok := ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
				Expect(ok).To(BeTrue())
				Expect(v).To(Equal(subnetName))

				Expect(ipPoolT.Spec.Gateway).To(Equal(subnetT.Spec.Gateway))
				Expect(ipPoolT.Spec.Vlan).To(Equal(subnetT.Spec.Vlan))
				Expect(ipPoolT.Spec.Routes).To(Equal(subnetT.Spec.Routes))
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

					err := tracker.Add(ipPoolT)
					Expect(err).NotTo(HaveOccurred())

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("creates an IPPool with the same 'spec.subnet'", func() {
					existIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					existIPPoolT.Spec.Subnet = "172.18.40.0/24"
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, "172.18.41.1-172.18.41.2")

					err := tracker.Add(existIPPoolT)
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

					err := tracker.Add(existIPPoolT)
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

					err := tracker.Add(existIPPoolT)
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

			When("Validating 'spec.default'", func() {
				It("creates non-default IPv4 IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.Default = pointer.Bool(false)

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

				It("creates default IPv4 IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.Default = pointer.Bool(true)

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("Validating 'spec.ips'", func() {
				It("inputs invalid 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, constant.InvalidIPRange)

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

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("is a empty IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

				It("exists invalid IPPool in the cluster", func() {
					ipVersion := constant.IPv4
					subnet := "172.18.40.0/24"
					cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
					Expect(err).NotTo(HaveOccurred())
					Expect(cidr).NotTo(BeEmpty())

					existIPPoolT.Labels[constant.LabelIPPoolCIDR] = cidr
					existIPPoolT.Spec.IPVersion = pointer.Int64(ipVersion)
					existIPPoolT.Spec.Subnet = subnet
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, constant.InvalidIPRange)

					err = tracker.Add(existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(ipVersion)
					ipPoolT.Spec.Subnet = subnet
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
					ipVersion := constant.IPv4
					subnet := "172.18.40.0/24"
					cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
					Expect(err).NotTo(HaveOccurred())
					Expect(cidr).NotTo(BeEmpty())

					existIPPoolT.Labels[constant.LabelIPPoolCIDR] = cidr
					existIPPoolT.Spec.IPVersion = pointer.Int64(ipVersion)
					existIPPoolT.Spec.Subnet = subnet
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, "172.18.40.10")

					err = tracker.Add(existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(ipVersion)
					ipPoolT.Spec.Subnet = subnet
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

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("duplicate with 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Gateway = pointer.String("172.18.40.1")

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("excludes gateway address through 'spec.excludeIPs'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs, "172.18.40.1")
					ipPoolT.Spec.Gateway = pointer.String("172.18.40.1")

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("Validating 'spec.routes'", func() {
				It("inputs default route", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Routes = append(ipPoolT.Spec.Routes,
						spiderpoolv2beta1.Route{
							Dst: "0.0.0.0/0",
							Gw:  "172.18.40.1",
						},
					)

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs duplicate routes", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Routes = append(ipPoolT.Spec.Routes,
						spiderpoolv2beta1.Route{
							Dst: "192.168.40.0/24",
							Gw:  "172.18.40.1",
						},
						spiderpoolv2beta1.Route{
							Dst: "192.168.40.0/24",
							Gw:  "172.18.40.2",
						},
					)

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

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
						spiderpoolv2beta1.Route{
							Dst: constant.InvalidCIDR,
							Gw:  "172.18.40.1",
						},
					)

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
						spiderpoolv2beta1.Route{
							Dst: "192.168.40.0/24",
							Gw:  constant.InvalidIP,
						},
					)

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
						spiderpoolv2beta1.Route{
							Dst: "192.168.40.0/24",
							Gw:  "172.18.41.1",
						},
					)

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating the total IP addresses contained in the controller Subnet", func() {
				BeforeEach(func() {
					ipPoolWebhook.EnableSpiderSubnet = true
				})

				It("succeed to create orphan IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
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

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("is a empty IPPool", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					err := tracker.Add(subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"

					err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

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

					err := tracker.Add(subnetT)
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

			When("Validating 'spec.podAffinity'", func() {
				It("no podAffinity", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1")
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

				Context("auto-created IPPool", func() {
					var autoPool *spiderpoolv2beta1.SpiderIPPool
					BeforeEach(func() {
						autoPool = ipPoolT.DeepCopy()
						autoPool.Spec.IPs = append(autoPool.Spec.IPs, "172.18.40.1")
						autoPool.Spec.Subnet = "172.18.40.0/24"
						autoPool.Spec.IPVersion = pointer.Int64(constant.IPv4)
						autoPool.Labels = map[string]string{
							constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(appsv1.SchemeGroupVersion.String()),
							constant.LabelIPPoolOwnerApplicationKind:      constant.KindDeployment,
							constant.LabelIPPoolOwnerApplicationNamespace: "test-ns",
							constant.LabelIPPoolOwnerApplicationName:      "test-name",
						}

						podController := types.PodTopController{
							AppNamespacedName: types.AppNamespacedName{
								APIVersion: appsv1.SchemeGroupVersion.String(),
								Kind:       constant.KindDeployment,
								Namespace:  "test-ns",
								Name:       "test-name",
							},
							UID: uuid.NewUUID(),
							APP: nil,
						}
						autoPool.Spec.PodAffinity = ippoolmanager.NewAutoPoolPodAffinity(podController)
					})

					It("auto-created IPPool with owner application deployment", func() {
						err := ipPoolWebhook.ValidateCreate(ctx, autoPool)
						Expect(err).NotTo(HaveOccurred())
					})

					It("auto-created IPPool with modified podAffinity", func() {
						autoPool.Spec.PodAffinity = &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key": "value",
							},
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{Key: "key",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"value"},
								},
							},
						}

						err := ipPoolWebhook.ValidateCreate(ctx, autoPool)
						Expect(err).To(HaveOccurred())
					})
				})

				Context("normal IPPool", func() {
					It("valid podAffinity", func() {
						ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
						ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1")
						ipPoolT.Spec.Subnet = "172.18.40.0/24"
						ipPoolT.Spec.PodAffinity = &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key": "value",
							},
							MatchExpressions: nil,
						}
						err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
						Expect(err).NotTo(HaveOccurred())
					})

					It("invalid podAffinity with invalid label value", func() {
						ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
						ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1")
						ipPoolT.Spec.Subnet = "172.18.40.0/24"
						ipPoolT.Spec.PodAffinity = &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"key": ".starts.with.dot",
							},
							MatchExpressions: nil,
						}
						err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
						Expect(err).To(HaveOccurred())
					})

					It("empty podAffinity is invalid", func() {
						ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
						ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1")
						ipPoolT.Spec.Subnet = "172.18.40.0/24"
						ipPoolT.Spec.PodAffinity = &metav1.LabelSelector{
							MatchLabels:      map[string]string{},
							MatchExpressions: []metav1.LabelSelectorRequirement{},
						}
						err := ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
						Expect(err).To(HaveOccurred())
					})
				})
			})

			It("creates IPv4 IPPool with all fields valid", func() {
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

				err := tracker.Add(subnetT)
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
					spiderpoolv2beta1.Route{
						Dst: "192.168.40.0/24",
						Gw:  "172.18.40.40",
					},
				)
				ipPoolT.Spec.Default = pointer.Bool(true)

				err = ipPoolWebhook.ValidateCreate(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("creates IPv6 IPPool with all fields valid", func() {
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

				err := tracker.Add(subnetT)
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
					spiderpoolv2beta1.Route{
						Dst: "fd00:40::/120",
						Gw:  "abcd:1234::28",
					},
				)
				ipPoolT.Spec.Default = pointer.Bool(true)

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

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.default'", func() {
				It("set default IPv4 IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.Default = pointer.Bool(false)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Default = pointer.Bool(true)

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("Validating 'spec.ips'", func() {
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

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("appends IP range that do not pertains to 'spec.subnet' to 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.41.10")

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("remove all 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = []string{}

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

				It("exists invalid IPPool in the cluster", func() {
					ipVersion := constant.IPv4
					subnet := "172.18.40.0/24"
					cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
					Expect(err).NotTo(HaveOccurred())
					Expect(cidr).NotTo(BeEmpty())

					existIPPoolT.Labels[constant.LabelIPPoolCIDR] = cidr
					existIPPoolT.Spec.IPVersion = pointer.Int64(ipVersion)
					existIPPoolT.Spec.Subnet = subnet
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, constant.InvalidIPRange)

					err = tracker.Add(existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(ipVersion)
					ipPoolT.Spec.Subnet = subnet
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("overlaps with existing IPPool", func() {
					ipVersion := constant.IPv4
					subnet := "172.18.40.0/24"
					cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
					Expect(err).NotTo(HaveOccurred())
					Expect(cidr).NotTo(BeEmpty())

					existIPPoolT.Labels[constant.LabelIPPoolCIDR] = cidr
					existIPPoolT.Spec.IPVersion = pointer.Int64(ipVersion)
					existIPPoolT.Spec.Subnet = subnet
					existIPPoolT.Spec.IPs = append(existIPPoolT.Spec.IPs, "172.18.40.10")

					err = tracker.Add(existIPPoolT)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(ipVersion)
					ipPoolT.Spec.Subnet = subnet
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
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Gateway = pointer.String(constant.InvalidIP)

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates 'spec.gateway' to a gateway that do not pertains to 'spec.subnet'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Gateway = pointer.String("172.18.41.1")

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("duplicate with 'spec.ips'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Gateway = pointer.String("172.18.40.1")

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("excludes gateway address through 'spec.excludeIPs'", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs, "172.18.40.1")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Gateway = pointer.String("172.18.40.1")

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("Validating 'spec.routes'", func() {
				It("appends default route", func() {
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
						spiderpoolv2beta1.Route{
							Dst: "0.0.0.0/0",
							Gw:  "172.18.40.1",
						},
					)

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("appends duplicate route", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
						[]string{
							"172.18.40.2-172.18.40.3",
							"172.18.40.10",
						}...,
					)
					ipPoolT.Spec.Routes = append(ipPoolT.Spec.Routes,
						spiderpoolv2beta1.Route{
							Dst: "192.168.40.0/24",
							Gw:  "172.18.40.1",
						},
					)

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.Routes = append(newIPPoolT.Spec.Routes,
						spiderpoolv2beta1.Route{
							Dst: "192.168.40.0/24",
							Gw:  "172.18.40.2",
						},
					)

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

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
						spiderpoolv2beta1.Route{
							Dst: constant.InvalidCIDR,
							Gw:  "172.18.40.1",
						},
					)

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
						spiderpoolv2beta1.Route{
							Dst: "192.168.40.0/24",
							Gw:  constant.InvalidIP,
						},
					)

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
						spiderpoolv2beta1.Route{
							Dst: "192.168.40.0/24",
							Gw:  "172.18.41.1",
						},
					)

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating the IP addresses being used", func() {
				It("removes IP range that is being used by IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(subnetT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					data, err := convert.MarshalIPPoolAllocatedIPs(
						spiderpoolv2beta1.PoolIPAllocations{
							"172.18.40.10": spiderpoolv2beta1.PoolIPAllocation{
								NIC:            "eth0",
								NamespacedName: "default/pod",
								PodUID:         string(uuid.NewUUID()),
							},
						},
					)
					Expect(err).NotTo(HaveOccurred())
					ipPoolT.Status.AllocatedIPs = data

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = newIPPoolT.Spec.IPs[:1]

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating the total IP addresses contained in the controller Subnet", func() {
				BeforeEach(func() {
					ipPoolWebhook.EnableSpiderSubnet = true
				})

				It("succeed to update orphan IPPool", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.40.10")

					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(err).NotTo(HaveOccurred())
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

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("remove all 'spec.ips' of IPPool", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.1-172.18.40.2")

					err := tracker.Add(subnetT)
					Expect(err).NotTo(HaveOccurred())

					err = controllerutil.SetControllerReference(subnetT, ipPoolT, scheme)
					Expect(err).NotTo(HaveOccurred())

					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.Subnet = "172.18.40.0/24"
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newIPPoolT := ipPoolT.DeepCopy()
					newIPPoolT.Spec.IPs = []string{}

					err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
					Expect(err).NotTo(HaveOccurred())
				})

				It("is out of the IP range of the Subnet", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.1-172.18.40.2")

					err := tracker.Add(subnetT)
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

				It("update auto-created IPPool by hand", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.1-172.18.40.2")

					autoPool := ipPoolT.DeepCopy()
					autoPool.Labels = map[string]string{
						constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(appsv1.SchemeGroupVersion.String()),
						constant.LabelIPPoolOwnerApplicationKind:      constant.KindDeployment,
						constant.LabelIPPoolOwnerApplicationNamespace: "test-ns",
						constant.LabelIPPoolOwnerApplicationName:      "test-name",
					}
					autoPool.Spec.IPVersion = pointer.Int64(constant.IPv4)
					autoPool.Spec.Subnet = "172.18.40.0/24"
					autoPool.Spec.IPs = append(autoPool.Spec.IPs, "172.18.40.1")

					err := controllerutil.SetControllerReference(subnetT, autoPool, scheme)
					Expect(err).NotTo(HaveOccurred())
					poolIPPreAllocations := spiderpoolv2beta1.PoolIPPreAllocations{autoPool.Name: spiderpoolv2beta1.PoolIPPreAllocation{
						IPs: []string{"172.18.40.1"},
						Application: pointer.String(applicationinformers.ApplicationNamespacedName(types.AppNamespacedName{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       constant.KindDeployment,
							Namespace:  "test-ns",
							Name:       "test-name",
						})),
					}}
					subnetAllocatedIPPools, err := convert.MarshalSubnetAllocatedIPPools(poolIPPreAllocations)
					Expect(err).NotTo(HaveOccurred())
					subnetT.Status.ControlledIPPools = subnetAllocatedIPPools

					err = tracker.Add(subnetT)
					Expect(err).NotTo(HaveOccurred())

					newAutoPool := autoPool.DeepCopy()
					newAutoPool.Spec.IPs = append(newAutoPool.Spec.IPs, "172.18.40.2")

					err = ipPoolWebhook.ValidateUpdate(ctx, autoPool, newAutoPool)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("update auto-created IPPool annotation", func() {
					subnetT.SetUID(uuid.NewUUID())
					subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					subnetT.Spec.Subnet = "172.18.40.0/24"
					subnetT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.1-172.18.40.2")

					autoPool := ipPoolT.DeepCopy()
					autoPool.Labels = map[string]string{
						constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(appsv1.SchemeGroupVersion.String()),
						constant.LabelIPPoolOwnerApplicationKind:      constant.KindDeployment,
						constant.LabelIPPoolOwnerApplicationNamespace: "test-ns",
						constant.LabelIPPoolOwnerApplicationName:      "test-name",
					}
					autoPool.Spec.IPVersion = pointer.Int64(constant.IPv4)
					autoPool.Spec.Subnet = "172.18.40.0/24"
					autoPool.Spec.IPs = append(autoPool.Spec.IPs, "172.18.40.1")

					err := controllerutil.SetControllerReference(subnetT, autoPool, scheme)
					Expect(err).NotTo(HaveOccurred())
					poolIPPreAllocations := spiderpoolv2beta1.PoolIPPreAllocations{autoPool.Name: spiderpoolv2beta1.PoolIPPreAllocation{
						IPs: []string{"172.18.40.1"},
						Application: pointer.String(applicationinformers.ApplicationNamespacedName(types.AppNamespacedName{
							APIVersion: appsv1.SchemeGroupVersion.String(),
							Kind:       constant.KindDeployment,
							Namespace:  "test-ns",
							Name:       "test-name",
						})),
					}}
					subnetAllocatedIPPools, err := convert.MarshalSubnetAllocatedIPPools(poolIPPreAllocations)
					Expect(err).NotTo(HaveOccurred())
					subnetT.Status.ControlledIPPools = subnetAllocatedIPPools

					err = tracker.Add(subnetT)
					Expect(err).NotTo(HaveOccurred())

					newAutoPool := autoPool.DeepCopy()
					anno := newAutoPool.GetAnnotations()
					if anno == nil {
						anno = make(map[string]string)
					}
					anno["aaa"] = "test"
					newAutoPool.Annotations = anno

					err = ipPoolWebhook.ValidateUpdate(ctx, autoPool, newAutoPool)
					Expect(err).NotTo(HaveOccurred())
				})
			})

			When("Validating 'spec.podAffinity'", func() {
				It("no podAffinity", func() {
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.1")
					ipPoolT.Spec.Subnet = "172.18.40.0/24"

					newPool := ipPoolT.DeepCopy()
					newPool.Spec.PodAffinity = &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"key": ".starts.with.dot",
						},
						MatchExpressions: nil,
					}
					err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newPool)
					Expect(err).To(HaveOccurred())
				})
			})

			It("deletes IPPool", func() {
				controllerutil.AddFinalizer(ipPoolT, constant.SpiderFinalizer)
				now := metav1.Now()
				ipPoolT.SetDeletionTimestamp(&now)
				ipPoolT.SetDeletionGracePeriodSeconds(pointer.Int64(0))

				newIPPoolT := ipPoolT.DeepCopy()
				controllerutil.RemoveFinalizer(newIPPoolT, constant.SpiderFinalizer)

				err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates terminating Subnet", func() {
				controllerutil.AddFinalizer(ipPoolT, constant.SpiderFinalizer)
				now := metav1.Now()
				ipPoolT.SetDeletionTimestamp(&now)
				ipPoolT.SetDeletionGracePeriodSeconds(pointer.Int64(30))

				newIPPoolT := ipPoolT.DeepCopy()

				err := ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("updates IPv4 IPPool with all fields valid", func() {
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

				err := tracker.Add(subnetT)
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
					spiderpoolv2beta1.Route{
						Dst: "192.168.40.0/24",
						Gw:  "172.18.40.40",
					},
				)

				err = ipPoolWebhook.ValidateUpdate(ctx, ipPoolT, newIPPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates IPv6 IPPool with all fields valid", func() {
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

				err := tracker.Add(subnetT)
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
					spiderpoolv2beta1.Route{
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
				err := ipPoolWebhook.ValidateDelete(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
