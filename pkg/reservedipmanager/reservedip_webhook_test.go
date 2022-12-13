// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
)

var _ = Describe("ReservedIPWebhook", Label("reservedip_webhook_test"), func() {
	Describe("Set up ReservedIPWebhook", func() {
		PIt("talks to a Kubernetes API server", func() {
			cfg, err := config.GetConfig()
			Expect(err).NotTo(HaveOccurred())
			Expect(cfg).NotTo(BeNil())

			mgr, err := ctrl.NewManager(cfg, manager.Options{Scheme: scheme})
			Expect(err).NotTo(HaveOccurred())

			err = rIPWebhook.SetupWebhookWithManager(mgr)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Test ReservedIPWebhook's method", func() {
		var rIPT *spiderpoolv1.SpiderReservedIP
		var rIPListT *spiderpoolv1.SpiderReservedIPList

		BeforeEach(func() {
			reservedipmanager.WebhookLogger = logutils.Logger.Named("ReservedIP-Webhook")
			rIPT = &spiderpoolv1.SpiderReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderReservedIPKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "reservedip",
				},
				Spec: spiderpoolv1.ReservedIPSpec{},
			}

			rIPListT = &spiderpoolv1.SpiderReservedIPList{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderReservedIPListKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ListMeta: metav1.ListMeta{},
			}
		})

		Describe("Default", func() {
			It("avoids modifying the terminating ReservedIP", func() {
				deletionTimestamp := metav1.NewTime(time.Now().Add(30 * time.Second))
				rIPT.SetDeletionTimestamp(&deletionTimestamp)

				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("avoids modifying the ReservedIP whose 'spec.ips' is empty", func() {
				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("failed to set 'spec.ipVersion' due to the first IP range of 'spec.ips' is invalid", func() {
				rIPT.Spec.IPs = append(rIPT.Spec.IPs, constant.InvalidIPRange)

				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPT.Spec.IPVersion).To(BeNil())
			})

			It("sets 'spec.ipVersion' to 4", func() {
				rIPT.Spec.IPs = append(rIPT.Spec.IPs, "172.18.40.1-172.18.40.2")

				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
				Expect(*rIPT.Spec.IPVersion).To(Equal(constant.IPv4))
			})

			It("sets 'spec.ipVersion' to 6", func() {
				rIPT.Spec.IPs = append(rIPT.Spec.IPs, "abcd:1234::1-abcd:1234::2")

				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
				Expect(*rIPT.Spec.IPVersion).To(Equal(constant.IPv6))
			})

			It("failed to merge 'spec.ips' due to the invalid 'spec.ipVersion'", func() {
				ipVersion := constant.InvalidIPVersion
				rIPT.Spec.IPVersion = &ipVersion
				rIPT.Spec.IPs = append(rIPT.Spec.IPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPT.Spec.IPs).To(Equal(
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					},
				))
			})

			It("failed to merge 'spec.ips' due to the invalid 'spec.ips'", func() {
				ipv4 := constant.IPv4
				rIPT.Spec.IPVersion = &ipv4
				rIPT.Spec.IPs = append(rIPT.Spec.IPs,
					[]string{
						constant.InvalidIPRange,
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPT.Spec.IPs).To(Equal(
					[]string{
						constant.InvalidIPRange,
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					},
				))
			})

			It("merges IPv4 'spec.ips'", func() {
				ipv4 := constant.IPv4
				rIPT.Spec.IPVersion = &ipv4
				rIPT.Spec.IPs = append(rIPT.Spec.IPs,
					[]string{
						"172.18.40.10",
						"172.18.40.1-172.18.40.2",
						"172.18.40.2-172.18.40.3",
					}...,
				)

				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPT.Spec.IPs).To(Equal(
					[]string{
						"172.18.40.1-172.18.40.3",
						"172.18.40.10",
					},
				))
			})

			It("merges IPv6 'spec.ips'", func() {
				ipv6 := constant.IPv6
				rIPT.Spec.IPVersion = &ipv6
				rIPT.Spec.IPs = append(rIPT.Spec.IPs,
					[]string{
						"abcd:1234::a",
						"abcd:1234::1-abcd:1234::2",
						"abcd:1234::2-abcd:1234::3",
					}...,
				)

				ctx := context.TODO()
				err := rIPWebhook.Default(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
				Expect(rIPT.Spec.IPs).To(Equal(
					[]string{
						"abcd:1234::1-abcd:1234::3",
						"abcd:1234::a",
					},
				))
			})
		})

		Describe("ValidateCreate", func() {
			BeforeEach(func() {
				rIPWebhook.EnableIPv4 = true
				rIPWebhook.EnableIPv6 = true
			})

			When("Validating 'spec.ipVersion'", func() {
				It("inputs nil 'spec.ipVersion'", func() {
					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).To(HaveOccurred())
				})

				It("inputs invalid 'spec.ipVersion'", func() {
					ipVersion := constant.InvalidIPVersion
					rIPT.Spec.IPVersion = &ipVersion

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).To(HaveOccurred())
				})

				It("creates IPv4 ReservedIP but IPv4 is disbale'", func() {
					rIPWebhook.EnableIPv4 = false
					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).To(HaveOccurred())
				})

				It("creates IPv6 ReservedIP but IPv6 is disbale'", func() {
					rIPWebhook.EnableIPv6 = false
					ipVersion := constant.IPv6
					rIPT.Spec.IPVersion = &ipVersion

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).To(HaveOccurred())
				})
			})

			When("Validating 'spec.ips'", func() {
				It("inputs empty 'spec.ips'", func() {
					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).NotTo(HaveOccurred())
				})

				It("inputs invalid 'spec.ips'", func() {
					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion
					rIPT.Spec.IPs = append(rIPT.Spec.IPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).To(HaveOccurred())
				})

				It("failed to list ReservedIPs due to some unknown errors", func() {
					mockRIPManager.EXPECT().
						ListReservedIPs(gomock.All()).
						Return(nil, constant.ErrUnknown).
						Times(1)

					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion
					rIPT.Spec.IPs = append(rIPT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).To(HaveOccurred())
				})

				It("exists invalid ReservedIPs in the cluster", func() {
					existRIPT := rIPT.DeepCopy()
					existRIPT.Name = "exist-reservedip"
					ipVersion := constant.IPv4
					existRIPT.Spec.IPVersion = &ipVersion
					existRIPT.Spec.IPs = append(existRIPT.Spec.IPs, constant.InvalidIPRange)
					rIPListT.Items = append(rIPListT.Items, *existRIPT)

					mockRIPManager.EXPECT().
						ListReservedIPs(gomock.All()).
						Return(rIPListT, nil).
						Times(1)

					rIPT.Spec.IPVersion = &ipVersion
					rIPT.Spec.IPs = append(rIPT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).To(HaveOccurred())
				})

				It("overlaps with the existing ReservedIP", func() {
					existRIPT := rIPT.DeepCopy()
					existRIPT.Name = "exist-reservedip"
					ipVersion := constant.IPv4
					existRIPT.Spec.IPVersion = &ipVersion
					existRIPT.Spec.IPs = append(existRIPT.Spec.IPs, "172.18.40.10")
					rIPListT.Items = append(rIPListT.Items, *existRIPT)

					mockRIPManager.EXPECT().
						ListReservedIPs(gomock.All()).
						Return(rIPListT, nil).
						Times(1)

					rIPT.Spec.IPVersion = &ipVersion
					rIPT.Spec.IPs = append(rIPT.Spec.IPs,
						[]string{
							"172.18.40.1-172.18.40.2",
							"172.18.40.10",
						}...,
					)

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(err).To(HaveOccurred())
				})
			})

			It("creates IPv4 ReservedIP with all fields valid", func() {
				mockRIPManager.EXPECT().
					ListReservedIPs(gomock.All()).
					Return(rIPListT, nil).
					Times(1)

				ipVersion := constant.IPv4
				rIPT.Spec.IPVersion = &ipVersion
				rIPT.Spec.IPs = append(rIPT.Spec.IPs,
					[]string{
						"172.18.40.1-172.18.40.2",
						"172.18.40.10",
					}...,
				)

				ctx := context.TODO()
				err := rIPWebhook.ValidateCreate(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("creates IPv6 ReservedIP with all fields valid", func() {
				mockRIPManager.EXPECT().
					ListReservedIPs(gomock.All()).
					Return(rIPListT, nil).
					Times(1)

				ipVersion := constant.IPv6
				rIPT.Spec.IPVersion = &ipVersion
				rIPT.Spec.IPs = append(rIPT.Spec.IPs,
					[]string{
						"abcd:1234::1-abcd:1234::2",
						"abcd:1234::a",
					}...,
				)

				ctx := context.TODO()
				err := rIPWebhook.ValidateCreate(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("ValidateUpdate", func() {
			BeforeEach(func() {
				rIPWebhook.EnableIPv4 = true
				rIPWebhook.EnableIPv6 = true
			})

			When("Validating 'spec.ipVersion'", func() {
				It("updates 'spec.ipVersion' to nil", func() {
					rIPWebhook.EnableIPv4 = false
					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPVersion = nil

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})

				It("updates 'spec.ipVersion' to invalid IP version", func() {
					rIPWebhook.EnableIPv4 = false
					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion

					newRIPT := rIPT.DeepCopy()
					invalidIPVersion := constant.InvalidIPVersion
					newRIPT.Spec.IPVersion = &invalidIPVersion

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})

				It("changes 'spec.ipVersion'", func() {
					rIPWebhook.EnableIPv4 = false
					ipv4 := constant.IPv4
					rIPT.Spec.IPVersion = &ipv4

					newRIPT := rIPT.DeepCopy()
					ipv6 := constant.IPv6
					newRIPT.Spec.IPVersion = &ipv6

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})

				It("updates IPv4 ReservedIP but IPv4 is disbale'", func() {
					rIPWebhook.EnableIPv4 = false
					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})

				It("updates IPv6 ReservedIP but IPv6 is disbale'", func() {
					rIPWebhook.EnableIPv6 = false
					ipVersion := constant.IPv6
					rIPT.Spec.IPVersion = &ipVersion

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "adbc:1234::a")

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})
			})

			When("Validating 'spec.ips'", func() {
				It("appends an invalid IP range to 'spec.ips'", func() {
					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})

				It("failed to list ReservedIPs due to some unknown errors", func() {
					mockRIPManager.EXPECT().
						ListReservedIPs(gomock.All()).
						Return(nil, constant.ErrUnknown).
						Times(1)

					ipVersion := constant.IPv4
					rIPT.Spec.IPVersion = &ipVersion
					rIPT.Spec.IPs = append(rIPT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})

				It("exists invalid ReservedIPs in the cluster", func() {
					existRIPT := rIPT.DeepCopy()
					existRIPT.Name = "exist-reservedip"
					ipVersion := constant.IPv4
					existRIPT.Spec.IPVersion = &ipVersion
					existRIPT.Spec.IPs = append(existRIPT.Spec.IPs, constant.InvalidIPRange)
					rIPListT.Items = append(rIPListT.Items, *existRIPT)

					mockRIPManager.EXPECT().
						ListReservedIPs(gomock.All()).
						Return(rIPListT, nil).
						Times(1)

					rIPT.Spec.IPVersion = &ipVersion
					rIPT.Spec.IPs = append(rIPT.Spec.IPs, "172.18.40.1-172.18.40.2")

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})

				It("overlaps with the existing ReservedIP", func() {
					existRIPT := rIPT.DeepCopy()
					existRIPT.Name = "exist-reservedip"
					ipVersion := constant.IPv4
					existRIPT.Spec.IPVersion = &ipVersion
					existRIPT.Spec.IPs = append(existRIPT.Spec.IPs, "172.18.40.10")

					rIPT.Spec.IPVersion = &ipVersion
					rIPT.Spec.IPs = append(rIPT.Spec.IPs, "172.18.40.1-172.18.40.2")
					rIPListT.Items = append(rIPListT.Items, *rIPT, *existRIPT)

					mockRIPManager.EXPECT().
						ListReservedIPs(gomock.All()).
						Return(rIPListT, nil).
						Times(1)

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(err).To(HaveOccurred())
				})
			})

			It("updates IPv4 ReservedIP with all fields valid", func() {
				mockRIPManager.EXPECT().
					ListReservedIPs(gomock.All()).
					Return(rIPListT, nil).
					Times(1)

				ipVersion := constant.IPv4
				rIPT.Spec.IPVersion = &ipVersion
				rIPT.Spec.IPs = append(rIPT.Spec.IPs, "172.18.40.1-172.18.40.2")

				newRIPT := rIPT.DeepCopy()
				newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "172.18.40.10")

				ctx := context.TODO()
				err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates IPv6 ReservedIP with all fields valid", func() {
				mockRIPManager.EXPECT().
					ListReservedIPs(gomock.All()).
					Return(rIPListT, nil).
					Times(1)

				ipVersion := constant.IPv6
				rIPT.Spec.IPVersion = &ipVersion
				rIPT.Spec.IPs = append(rIPT.Spec.IPs, "abcd:1234::1-abcd:1234::2")

				newRIPT := rIPT.DeepCopy()
				newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "abcd:1234::a")

				ctx := context.TODO()
				err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("ValidateDelete", func() {
			It("passes", func() {
				ctx := context.TODO()
				err := rIPWebhook.ValidateDelete(ctx, rIPT)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
