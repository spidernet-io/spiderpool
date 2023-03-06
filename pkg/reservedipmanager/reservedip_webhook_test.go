// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
		var count uint64
		var rIPName string
		var rIPT *spiderpoolv1.SpiderReservedIP

		BeforeEach(func() {
			reservedipmanager.WebhookLogger = logutils.Logger.Named("ReservedIP-Webhook")
			rIPWebhook.EnableIPv4 = true
			rIPWebhook.EnableIPv6 = true

			atomic.AddUint64(&count, 1)
			rIPName = fmt.Sprintf("reservedip-%v", count)
			rIPT = &spiderpoolv1.SpiderReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderReservedIP,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: rIPName,
				},
				Spec: spiderpoolv1.ReservedIPSpec{},
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
			err := fakeClient.Delete(ctx, rIPT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("Default", func() {
			It("avoids modifying the terminating ReservedIP", func() {
				now := metav1.Now()
				rIPT.SetDeletionTimestamp(&now)

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
				rIPT.Spec.IPVersion = pointer.Int64(constant.InvalidIPVersion)
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
				rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)
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
				rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)
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
				rIPT.Spec.IPVersion = pointer.Int64(constant.IPv6)
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
			When("Validating 'spec.ipVersion'", func() {
				It("inputs nil 'spec.ipVersion'", func() {
					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("inputs invalid 'spec.ipVersion'", func() {
					rIPT.Spec.IPVersion = pointer.Int64(constant.InvalidIPVersion)

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("creates IPv4 ReservedIP but IPv4 is disbale'", func() {
					rIPWebhook.EnableIPv4 = false
					rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("creates IPv6 ReservedIP but IPv6 is disbale'", func() {
					rIPWebhook.EnableIPv6 = false
					rIPT.Spec.IPVersion = pointer.Int64(constant.IPv6)

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.ips'", func() {
				It("inputs invalid 'spec.ips'", func() {
					rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					rIPT.Spec.IPs = append(rIPT.Spec.IPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := rIPWebhook.ValidateCreate(ctx, rIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			It("creates IPv4 ReservedIP with all fields valid", func() {
				rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)
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
				rIPT.Spec.IPVersion = pointer.Int64(constant.IPv6)
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
			When("Validating 'spec.ipVersion'", func() {
				It("updates 'spec.ipVersion' to nil", func() {
					rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPVersion = nil

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("changes 'spec.ipVersion'", func() {
					rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPVersion = pointer.Int64(constant.IPv6)

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates IPv4 ReservedIP but IPv4 is disbale'", func() {
					rIPWebhook.EnableIPv4 = false
					rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "172.18.40.10")

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})

				It("updates IPv6 ReservedIP but IPv6 is disbale'", func() {
					rIPWebhook.EnableIPv6 = false
					rIPT.Spec.IPVersion = pointer.Int64(constant.IPv6)

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "adbc:1234::a")

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			When("Validating 'spec.ips'", func() {
				It("appends invalid IP range to 'spec.ips'", func() {
					rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)

					newRIPT := rIPT.DeepCopy()
					newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, constant.InvalidIPRange)

					ctx := context.TODO()
					err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
					Expect(apierrors.IsInvalid(err)).To(BeTrue())
				})
			})

			It("deletes ReservedIP", func() {
				newRIPT := rIPT.DeepCopy()
				now := metav1.Now()
				newRIPT.SetDeletionTimestamp(&now)
				newRIPT.SetDeletionGracePeriodSeconds(pointer.Int64(0))

				ctx := context.TODO()
				err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates terminating ReservedIP", func() {
				now := metav1.Now()
				rIPT.SetDeletionTimestamp(&now)
				rIPT.SetDeletionGracePeriodSeconds(pointer.Int64(30))
				newRIPT := rIPT.DeepCopy()

				ctx := context.TODO()
				err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("updates IPv4 ReservedIP with all fields valid", func() {
				rIPT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				rIPT.Spec.IPs = append(rIPT.Spec.IPs, "172.18.40.1-172.18.40.2")

				newRIPT := rIPT.DeepCopy()
				newRIPT.Spec.IPs = append(newRIPT.Spec.IPs, "172.18.40.10")

				ctx := context.TODO()
				err := rIPWebhook.ValidateUpdate(ctx, rIPT, newRIPT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("updates IPv6 ReservedIP with all fields valid", func() {
				rIPT.Spec.IPVersion = pointer.Int64(constant.IPv6)
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
