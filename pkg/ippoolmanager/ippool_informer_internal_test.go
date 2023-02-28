// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package ippoolmanager

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/moby/moby/pkg/stringid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager/controllers"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var _ = Describe("IPPoolInformer", Label("ippool_informer_test"), func() {
	var scheme *runtime.Scheme
	var fakeClient client.Client
	var ipPoolController *IPPoolController
	var count uint64
	var ipPoolName string
	var ipPoolT *spiderpoolv1.SpiderIPPool
	var rIPManager reservedipmanager.ReservedIPManager
	var subnetName string
	var subnetT *spiderpoolv1.SpiderSubnet
	var labels map[string]string

	Describe("Test IPPoolInformer's method", func() {
		BeforeEach(func() {
			scheme = runtime.NewScheme()
			err := spiderpoolv1.AddToScheme(scheme)
			Expect(err).NotTo(HaveOccurred())

			fakeClient = fake.NewClientBuilder().
				WithScheme(scheme).
				Build()

			rIPManager, err = reservedipmanager.NewReservedIPManager(fakeClient)
			Expect(err).NotTo(HaveOccurred())

			ipPoolController = NewIPPoolController(
				IPPoolControllerConfig{
					EnableIPv4:                    true,
					EnableIPv6:                    true,
					IPPoolControllerWorkers:       int(2),
					EnableSpiderSubnet:            true,
					LeaderRetryElectGap:           time.Duration(1) * time.Second,
					MaxWorkqueueLength:            int(3),
					WorkQueueRequeueDelayDuration: time.Duration(2) * time.Second,
					WorkQueueMaxRetries:           int(3),
					ResyncPeriod:                  time.Duration(2) * time.Second,
				},
				fakeClient,
				rIPManager,
			)
			ipPoolController.normalPoolWorkQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Normal-SpiderIPPools")
			Expect(ipPoolController).NotTo(BeNil())

			atomic.AddUint64(&count, 1)
			ipPoolName = fmt.Sprintf("IPPool-%v", count)
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
			atomic.AddUint64(&count, 1)
			subnetName = fmt.Sprintf("Subnet-%v", count)
			labels = map[string]string{"foo": fmt.Sprintf("bar-%v", count)}
			subnetT = &spiderpoolv1.SpiderSubnet{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderSubnetKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   subnetName,
					Labels: labels,
				},
				Spec: spiderpoolv1.SubnetSpec{},
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

			err = fakeClient.Delete(ctx, subnetT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		Describe("updateSpiderIPPool", func() {
			It("enqueueIPPool", func() {
				ipPoolController.enqueueIPPool(ipPoolT)
			})

			When("label 'ipam.spidernet.io/owner-application' exist", func() {
				BeforeEach(func() {
					labels := map[string]string{"ipam.spidernet.io/owner-application": ipPoolName}
					ipPoolT.Labels = labels
				})

				It("pool.Status.AutoDesiredIPCount is nil ", func() {
					ipPoolT.Status.AutoDesiredIPCount = nil

					ctx := context.TODO()
					err := fakeClient.Create(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
					ipPoolController.enqueueIPPool(ipPoolT)
				})

				It("pool.Status.AutoDesiredIPCount is not nil ", func() {
					ipPoolController.v4AutoPoolWorkQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AutoCreated-SpiderIPPools-IPv4")
					ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(1)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
					ipPoolController.enqueueIPPool(ipPoolT)
				})

				It("pool.Spec.IPVersion is not nil ", func() {
					ipPoolController.v4AutoPoolWorkQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AutoCreated-SpiderIPPools-IPv4")
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
					ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(1)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
					ipPoolController.enqueueIPPool(ipPoolT)
				})

				It("pool.Spec.IPVersion not equal to constant.IPv4 ", func() {
					ipPoolController.v6AutoPoolWorkQueue = workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "AutoCreated-SpiderIPPools-IPv6")
					ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)
					ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(1)

					ctx := context.TODO()
					err := fakeClient.Create(ctx, ipPoolT)
					Expect(err).NotTo(HaveOccurred())
					ipPoolController.enqueueIPPool(ipPoolT)
				})
			})
		})

		It("onIPPoolAdd", func() {
			ipPoolController.onIPPoolAdd(ipPoolT)
		})

		Describe("updateSpiderIPPool", func() {
			It("IPPool' DeletionTimestamp is nil", func() {
				ipPoolT.SetUID(uuid.NewUUID())
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.10")

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				newIPPoolT := ipPoolT.DeepCopy()
				newIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				now := metav1.Now()
				newIPPoolT.SetDeletionTimestamp(&now)
				err = ipPoolController.updateSpiderIPPool(ipPoolT, newIPPoolT, InformerLogger.With(zap.String("onIPPoolAdd", newIPPoolT.Name)))
				Expect(err).NotTo(HaveOccurred())
			})

			It("IPPool' TotalIPCount not nil", func() {
				ipPoolT.SetUID(uuid.NewUUID())
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.10")

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				newIPPoolT := ipPoolT.DeepCopy()
				newIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				newIPPoolT.Status.TotalIPCount = pointer.Int64(10)

				err = ipPoolController.updateSpiderIPPool(ipPoolT, newIPPoolT, InformerLogger.With(zap.String("onIPPoolAdd", newIPPoolT.Name)))
				Expect(err).NotTo(HaveOccurred())
			})

			It("The Spec.IPs of the new and old pools are not equal", func() {
				ipPoolT.SetUID(uuid.NewUUID())
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.10")

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				newIPPoolT := ipPoolT.DeepCopy()
				newIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				newIPPoolT.Spec.Subnet = "172.18.41.0/24"
				newIPPoolT.Spec.IPs = append(newIPPoolT.Spec.IPs, "172.18.41.10")

				err = ipPoolController.updateSpiderIPPool(ipPoolT, newIPPoolT, InformerLogger.With(zap.String("onIPPoolAdd", newIPPoolT.Name)))
				Expect(err).NotTo(HaveOccurred())
			})

			It("The Spec.ExcludeIP of the old pool and the new pool are not equal", func() {
				ipPoolT.SetUID(uuid.NewUUID())
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs, "172.18.40.10")

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				newIPPoolT := ipPoolT.DeepCopy()
				newIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				newIPPoolT.Spec.Subnet = "172.18.41.0/24"
				newIPPoolT.Spec.ExcludeIPs = append(newIPPoolT.Spec.ExcludeIPs, "172.18.41.10")

				err = ipPoolController.updateSpiderIPPool(ipPoolT, newIPPoolT, InformerLogger.With(zap.String("onIPPoolAdd", newIPPoolT.Name)))
				Expect(err).NotTo(HaveOccurred())
			})

			It("TotalIPCount does not need to be updated", func() {
				ipPoolT.SetUID(uuid.NewUUID())
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Subnet = "172.18.40.0/24"
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs, "172.18.40.10")

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				newIPPoolT := ipPoolT.DeepCopy()
				newIPPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				newIPPoolT.Status.AllocatedIPCount = pointer.Int64(1)
				newIPPoolT.Status.TotalIPCount = pointer.Int64(1)
				newIPPoolT.Status.AllocatedIPs = nil
				err = ipPoolController.updateSpiderIPPool(ipPoolT, newIPPoolT, InformerLogger.With(zap.String("onIPPoolAdd", newIPPoolT.Name)))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("scaleIPPoolIfNeeded", func() {

			BeforeEach(func() {
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Vlan = pointer.Int64(0)
				ipPoolT.Spec.Subnet = "172.19.41.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.19.41.2-172.19.41.3",
					}...,
				)
				ipPoolT.Status.AllocatedIPCount = pointer.Int64(2)

				ipVersion := constant.IPv4
				subnet := "172.18.41.0/24"
				subnetT.Spec.IPVersion = &ipVersion
				subnetT.Spec.Subnet = subnet
				subnetT.Spec.IPs = append(subnetT.Spec.IPs,
					[]string{
						"172.19.41.2-172.19.41.10",
					}...,
				)
			})

			It("constant.LabelIPPoolOwnerSpiderSubnet does not exists", func() {
				ctx := context.TODO()
				err := ipPoolController.scaleIPPoolIfNeeded(ctx, ipPoolT)
				Expect(err).To(MatchError(constant.ErrWrongInput))
			})

			It("constant.LabelIPPoolOwnerSpiderSubnet exists", func() {
				labels := map[string]string{constant.LabelIPPoolOwnerSpiderSubnet: subnetName}
				ipPoolT.Labels = labels
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(3)
				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolController.scaleIPPoolIfNeeded(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("AutoDesiredIPCount is nil", func() {
				labels := map[string]string{constant.LabelIPPoolOwnerSpiderSubnet: subnetName}
				ipPoolT.Labels = labels
				ipPoolT.Status.AllocatedIPCount = nil

				ctx := context.TODO()
				err := ipPoolController.scaleIPPoolIfNeeded(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("AssembleTotalIPs return error", func() {
				labels := map[string]string{constant.LabelIPPoolOwnerSpiderSubnet: subnetName}
				ipPoolT.Labels = labels
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(1)
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						" 172.19.41.3 - 172.19.41.100 ",
					}...,
				)
				ipPoolT.Spec.ExcludeIPs = append(ipPoolT.Spec.ExcludeIPs,
					[]string{
						" 172.19.41.90 - 172.19.41.100 ",
					}...,
				)

				ctx := context.TODO()
				err := ipPoolController.scaleIPPoolIfNeeded(ctx, ipPoolT)
				Expect(err).To(MatchError(constant.ErrWrongInput))
			})

			It("desiredIPNum == totalIPCount", func() {
				labels := map[string]string{constant.LabelIPPoolOwnerSpiderSubnet: subnetName}
				ipPoolT.Labels = labels
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.19.41.3",
					}...,
				)
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(1)

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolController.scaleIPPoolIfNeeded(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("desiredIPNum > totalIPCount", func() {
				labels := map[string]string{constant.LabelIPPoolOwnerSpiderSubnet: subnetName}
				ipPoolT.Labels = labels
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(3)
				ipPoolController.v4GenIPsCursor = true

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolController.scaleIPPoolIfNeeded(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("desiredIPNum > totalIPCount and ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)", func() {
				labels := map[string]string{constant.LabelIPPoolOwnerSpiderSubnet: subnetName}
				ipPoolT.Labels = labels
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(3)
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv6)
				ipPoolT.Spec.IPs = []string{"abcd:1234::2-abcd:1234::9"}
				ipPoolT.Spec.Subnet = "abcd:1234::/120"
				ipPoolController.v6GenIPsCursor = true

				subnetT.Spec.IPVersion = pointer.Int64(constant.IPv6)
				subnetT.Spec.IPs = []string{"abcd:1234::2-abcd:1234::9"}
				subnetT.Spec.Subnet = "abcd:1234::/120"

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolController.scaleIPPoolIfNeeded(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})

			It("desiredIPNum <= totalIPCount", func() {
				labels := map[string]string{constant.LabelIPPoolOwnerSpiderSubnet: subnetName}
				ipPoolT.Labels = labels
				ipPoolT.Status.AutoDesiredIPCount = pointer.Int64(3)
				ipPoolT.Spec.IPs = []string{"172.19.41.2-172.19.41.3"}

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				err = fakeClient.Create(ctx, subnetT)
				Expect(err).NotTo(HaveOccurred())

				err = ipPoolController.scaleIPPoolIfNeeded(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Describe("cleanAutoIPPoolLegacy", func() {
			BeforeEach(func() {
				ipPoolT.SetUID(uuid.NewUUID())
				ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
				ipPoolT.Spec.Vlan = pointer.Int64(0)
				ipPoolT.Spec.Subnet = "172.19.41.0/24"
				ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs,
					[]string{
						"172.19.41.2-172.19.41.3",
					}...,
				)
			})

			It("pool.DeletionTimestamp != nil", func() {
				now := metav1.Now()
				ipPoolT.SetDeletionTimestamp(&now)

				ctx := context.TODO()
				hasClean, err := ipPoolController.cleanAutoIPPoolLegacy(ctx, ipPoolT)
				Expect(hasClean).To(BeTrue())
				Expect(err).NotTo(HaveOccurred())
			})

			It("pool.Status.AllocatedIPs is not equal 0", func() {
				ipNumber := "172.19.41.2"
				containerID := stringid.GenerateRandomID()

				ipPoolT.Status.AllocatedIPs = spiderpoolv1.PoolIPAllocations{}
				allocation := spiderpoolv1.PoolIPAllocation{
					ContainerID: containerID,
				}
				ipPoolT.Status.AllocatedIPs[ipNumber] = allocation

				ctx := context.TODO()
				err := fakeClient.Create(ctx, ipPoolT)
				Expect(err).NotTo(HaveOccurred())

				hasClean, err := ipPoolController.cleanAutoIPPoolLegacy(ctx, ipPoolT)
				Expect(hasClean).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
			})

			It("isReclaim != constant.True", func() {
				labels := map[string]string{
					constant.LabelIPPoolReclaimIPPool: constant.False,
				}
				ipPoolT.Labels = labels
				ctx := context.TODO()
				hasClean, err := ipPoolController.cleanAutoIPPoolLegacy(ctx, ipPoolT)
				Expect(hasClean).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
			})

			It("LabelIPPoolOwnerApplication does not exist", func() {
				labels := map[string]string{
					constant.LabelIPPoolReclaimIPPool: constant.True,
				}
				ipPoolT.Labels = labels
				ctx := context.TODO()
				hasClean, err := ipPoolController.cleanAutoIPPoolLegacy(ctx, ipPoolT)
				Expect(hasClean).To(BeFalse())
				Expect(err).To(MatchError(constant.ErrWrongInput))
			})

			It("LabelIPPoolOwnerApplication with daemonSet ", func() {
				daemonSet := &appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "Pod",
						Namespace: "default",
					},
				}

				labels := map[string]string{
					constant.LabelIPPoolReclaimIPPool:    constant.True,
					constant.LabelIPPoolOwnerApplication: controllers.AppLabelValue(daemonSet.Kind, daemonSet.Namespace, daemonSet.Name),
				}
				ipPoolT.Labels = labels
				GinkgoWriter.Printf("xxx %+v", labels)
				ctx := context.TODO()
				hasClean, err := ipPoolController.cleanAutoIPPoolLegacy(ctx, ipPoolT)
				Expect(hasClean).To(BeFalse())
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
