// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package subnetmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	spiderpoolfake "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned/fake"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
	"github.com/spidernet-io/spiderpool/pkg/subnetmanager"
	"github.com/spidernet-io/spiderpool/pkg/utils/convert"
)

var _ = Describe("SubnetController", Label("subnet_controller_test"), func() {
	Describe("SYNC", func() {
		var ctx context.Context

		var count uint64
		var subnetName string
		var subnetT *spiderpoolv1.SpiderSubnet
		var ipPoolName string
		var ipPoolT *spiderpoolv1.SpiderIPPool

		var subnetController *subnetmanager.SubnetController
		var fakeSubnetWatch, fakeIPPoolWatch *watch.FakeWatcher
		var subnetIndexer, ipPoolIndexer cache.Indexer

		BeforeEach(func() {
			subnetmanager.InformerLogger = logutils.Logger.Named("Subnet-Informer")

			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			subnetName = fmt.Sprintf("subnet-%v", count)
			subnetT = &spiderpoolv1.SpiderSubnet{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderSubnet,
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
					Kind:       constant.KindSpiderIPPool,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   ipPoolName,
					Labels: map[string]string{},
				},
				Spec: spiderpoolv1.IPPoolSpec{},
			}

			subnetController = &subnetmanager.SubnetController{
				Client:                  fakeClient,
				APIReader:               fakeClient,
				LeaderRetryElectGap:     time.Second,
				SubnetControllerWorkers: 1,
				MaxWorkqueueLength:      1,
			}

			patches := gomonkey.ApplyFuncReturn(cache.WaitForNamedCacheSync, true)
			DeferCleanup(patches.Reset)

			mockLeaderElector.EXPECT().
				IsElected().
				Return(true).
				AnyTimes()

			bCtx, cancel := context.WithCancel(context.Background())
			DeferCleanup(cancel)

			fakeSubnetWatch = watch.NewFake()
			fakeIPPoolWatch = watch.NewFake()
			fakeClientset := spiderpoolfake.NewSimpleClientset()
			fakeClientset.PrependWatchReactor("spidersubnets", testing.DefaultWatchReactor(fakeSubnetWatch, nil))
			fakeClientset.PrependWatchReactor("spiderippools", testing.DefaultWatchReactor(fakeIPPoolWatch, nil))

			err := subnetController.SetupInformer(bCtx, fakeClientset, mockLeaderElector)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				g.Expect(subnetController.SubnetIndexer).NotTo(BeNil())
				g.Expect(subnetController.IPPoolIndexer).NotTo(BeNil())
				subnetIndexer = subnetController.SubnetIndexer
				ipPoolIndexer = subnetController.IPPoolIndexer
			}).Should(Succeed())
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

			err = fakeClient.Delete(ctx, subnetT, deleteOption)
			Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
		})

		It("sets CIDR label", func() {
			ipVersion := constant.IPv4
			subnet := "172.18.40.0/24"
			cidr, err := spiderpoolip.CIDRToLabelValue(ipVersion, subnet)
			Expect(err).NotTo(HaveOccurred())
			Expect(cidr).NotTo(BeEmpty())

			subnetT.Spec.IPVersion = &ipVersion
			subnetT.Spec.Subnet = subnet

			err = fakeClient.Create(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnetT)
			Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Add(subnetT)
			Eventually(func(g Gomega) {
				var subnetR spiderpoolv1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())

				v, ok := subnetR.Labels[constant.LabelSubnetCIDR]
				g.Expect(ok).To(BeTrue())
				g.Expect(v).To(Equal(cidr))
			}).Should(Succeed())
		})

		It("sets the owner reference of the controlled IPPools", func() {
			subnetT.SetUID(uuid.NewUUID())
			subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
			subnetT.Spec.Subnet = "172.18.40.0/24"

			err := fakeClient.Create(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnetT)
			Expect(err).NotTo(HaveOccurred())

			ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
			ipPoolT.Spec.Subnet = "172.18.40.0/24"

			err = fakeClient.Create(ctx, ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolIndexer.Add(ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Add(subnetT)
			Eventually(func(g Gomega) {
				var subnetR spiderpoolv1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())

				var ipPoolR spiderpoolv1.SpiderIPPool
				err = fakeClient.Get(ctx, types.NamespacedName{Name: ipPoolT.Name}, &ipPoolR)
				g.Expect(err).NotTo(HaveOccurred())

				controlled := metav1.IsControlledBy(&ipPoolR, &subnetR)
				g.Expect(controlled).To(BeTrue())

				v, ok := ipPoolR.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
				g.Expect(ok).To(BeTrue())
				g.Expect(v).To(Equal(subnetR.Name))
			}).Should(Succeed())
		})

		It("aggregates pre-allocation status", func() {
			subnetT.SetUID(uuid.NewUUID())
			subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
			subnetT.Spec.Subnet = "172.18.40.0/24"
			subnetT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.10")

			err := fakeClient.Create(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnetT)
			Expect(err).NotTo(HaveOccurred())

			ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet] = subnetT.Name
			ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
			ipPoolT.Spec.Subnet = "172.18.40.0/24"
			ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.10")

			err = fakeClient.Create(ctx, ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolIndexer.Add(ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Add(subnetT)
			Eventually(func(g Gomega) {
				var subnetR spiderpoolv1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())
				subnetAllocatedIPPools, err := convert.UnmarshalSubnetAllocatedIPPools(subnetR.Status.ControlledIPPools)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(subnetAllocatedIPPools).To(Equal(
					spiderpoolv1.PoolIPPreAllocations{
						ipPoolT.Name: spiderpoolv1.PoolIPPreAllocation{
							IPs: []string{"172.18.40.10"},
						},
					},
				))
				g.Expect(*subnetR.Status.TotalIPCount).To(BeNumerically("==", 1))
				g.Expect(*subnetR.Status.AllocatedIPCount).To(BeNumerically("==", 1))
			}).Should(Succeed())
		})

		It("cascades delete Subnet and the IPPools it controls", func() {
			controllerutil.AddFinalizer(subnetT, constant.SpiderFinalizer)
			now := metav1.Now()
			subnetT.SetDeletionTimestamp(&now)
			subnetT.SetDeletionGracePeriodSeconds(pointer.Int64(0))
			subnetT.SetUID(uuid.NewUUID())
			subnetT.Spec.IPVersion = pointer.Int64(constant.IPv4)
			subnetT.Spec.Subnet = "172.18.40.0/24"

			err := fakeClient.Create(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnetT)
			Expect(err).NotTo(HaveOccurred())

			// TODO(iiiceoo): Depends on K8s GC.
			// ctrl.SetControllerReference(subnetT, ipPoolT, scheme)
			// ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet] = subnetT.Name
			// ipPoolT.Spec.IPVersion = pointer.Int64(constant.IPv4)
			// ipPoolT.Spec.Subnet = "172.18.40.0/24"

			// err = fakeClient.Create(ctx, ipPoolT)
			// Expect(err).NotTo(HaveOccurred())

			// err = ipPoolIndexer.Add(ipPoolT)
			// Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Modify(subnetT)
			Eventually(func(g Gomega) {
				// var ipPoolR spiderpoolv1.SpiderIPPool
				// err = fakeClient.Get(ctx, types.NamespacedName{Name: ipPoolT.Name}, &ipPoolR)
				// g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

				var subnetR spiderpoolv1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})
	})
})
