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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
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
		var subnetT *spiderpoolv2beta1.SpiderSubnet
		var ipPoolName string
		var ipPoolT *spiderpoolv2beta1.SpiderIPPool

		var subnetController *subnetmanager.SubnetController
		var fakeSubnetWatch, fakeIPPoolWatch *watch.FakeWatcher
		var subnetIndexer, ipPoolIndexer cache.Indexer

		BeforeEach(func() {
			subnetmanager.InformerLogger = logutils.Logger.Named("Subnet-Informer")

			ctx = context.TODO()

			atomic.AddUint64(&count, 1)
			subnetName = fmt.Sprintf("subnet-%v", count)
			subnetT = &spiderpoolv2beta1.SpiderSubnet{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.KindSpiderSubnet,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: subnetName,
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
					Name:   ipPoolName,
					Labels: map[string]string{},
				},
				Spec: spiderpoolv2beta1.IPPoolSpec{},
			}

			subnetController = &subnetmanager.SubnetController{
				Client:                  fakeClient,
				APIReader:               fakeClient,
				LeaderRetryElectGap:     time.Second,
				SubnetControllerWorkers: 1,
				MaxWorkqueueLength:      1,
				DynamicClient:           fakeDynamicClient,
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
				GracePeriodSeconds: ptr.To(int64(0)),
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
				var subnetR spiderpoolv2beta1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())

				v, ok := subnetR.Labels[constant.LabelSubnetCIDR]
				g.Expect(ok).To(BeTrue())
				g.Expect(v).To(Equal(cidr))
			}).Should(Succeed())
		})

		It("sets the owner reference of the controlled IPPools", func() {
			subnetT.SetUID(uuid.NewUUID())
			subnetT.Spec.IPVersion = ptr.To(constant.IPv4)
			subnetT.Spec.Subnet = "172.18.40.0/24"

			err := fakeClient.Create(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnetT)
			Expect(err).NotTo(HaveOccurred())

			ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
			ipPoolT.Spec.Subnet = "172.18.40.0/24"

			err = fakeClient.Create(ctx, ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolIndexer.Add(ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Add(subnetT)
			Eventually(func(g Gomega) {
				var subnetR spiderpoolv2beta1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())

				var ipPoolR spiderpoolv2beta1.SpiderIPPool
				err = fakeClient.Get(ctx, types.NamespacedName{Name: ipPoolT.Name}, &ipPoolR)
				g.Expect(err).NotTo(HaveOccurred())

				controlled := metav1.IsControlledBy(&ipPoolR, &subnetR)
				g.Expect(controlled).To(BeTrue())

				v, ok := ipPoolR.Labels[constant.LabelIPPoolOwnerSpiderSubnet]
				g.Expect(ok).To(BeTrue())
				g.Expect(v).To(Equal(subnetR.Name))
			}).Should(Succeed())
		})

		It("sets the owner reference for the orphan IPPool", func() {
			ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
			ipPoolT.Spec.Subnet = "172.18.40.0/24"

			err := fakeClient.Create(ctx, ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolIndexer.Add(ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			subnetT.SetUID(uuid.NewUUID())
			subnetT.Spec.IPVersion = ptr.To(constant.IPv4)
			subnetT.Spec.Subnet = "172.18.40.0/24"

			err = fakeClient.Create(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnetT)
			Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Add(subnetT)
			Eventually(func(g Gomega) {
				var subnetR spiderpoolv2beta1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())

				var ipPoolR spiderpoolv2beta1.SpiderIPPool
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
			subnetT.Spec.IPVersion = ptr.To(constant.IPv4)
			subnetT.Spec.Subnet = "172.18.40.0/24"
			subnetT.Spec.IPs = append(subnetT.Spec.IPs, "172.18.40.10")

			err := fakeClient.Create(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnetT)
			Expect(err).NotTo(HaveOccurred())

			ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet] = subnetT.Name
			ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
			ipPoolT.Spec.Subnet = "172.18.40.0/24"
			ipPoolT.Spec.IPs = append(ipPoolT.Spec.IPs, "172.18.40.10")

			err = fakeClient.Create(ctx, ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			err = ipPoolIndexer.Add(ipPoolT)
			Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Add(subnetT)
			Eventually(func(g Gomega) {
				var subnetR spiderpoolv2beta1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())

				preAllocations, err := convert.UnmarshalSubnetAllocatedIPPools(subnetR.Status.ControlledIPPools)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(preAllocations).To(Equal(
					spiderpoolv2beta1.PoolIPPreAllocations{
						ipPoolT.Name: spiderpoolv2beta1.PoolIPPreAllocation{
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
			subnetT.SetUID(uuid.NewUUID())
			subnetT.Spec.IPVersion = ptr.To(constant.IPv4)
			subnetT.Spec.Subnet = "172.18.40.0/24"

			err := fakeClient.Create(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = fakeClient.Delete(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, subnetT)
			Expect(err).NotTo(HaveOccurred())
			controllerutil.RemoveFinalizer(subnetT, constant.SpiderFinalizer)
			err = fakeClient.Update(ctx, subnetT)
			Expect(err).NotTo(HaveOccurred())

			// TODO(iiiceoo): Depends on K8s GC.
			// ctrl.SetControllerReference(subnetT, ipPoolT, scheme)
			// ipPoolT.Labels[constant.LabelIPPoolOwnerSpiderSubnet] = subnetT.Name
			// ipPoolT.Spec.IPVersion = ptr.To(constant.IPv4)
			// ipPoolT.Spec.Subnet = "172.18.40.0/24"

			// err = fakeClient.Create(ctx, ipPoolT)
			// Expect(err).NotTo(HaveOccurred())

			// err = ipPoolIndexer.Add(ipPoolT)
			// Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Modify(subnetT)
			Eventually(func(g Gomega) {
				// var ipPoolR spiderpoolv2beta1.SpiderIPPool
				// err = fakeClient.Get(ctx, types.NamespacedName{Name: ipPoolT.Name}, &ipPoolR)
				// g.Expect(apierrors.IsNotFound(err)).To(BeTrue())

				var subnetR spiderpoolv2beta1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnetT.Name}, &subnetR)
				g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			}).Should(Succeed())
		})

		It("third-party controller exist with auto-created IPPool", func() {
			subnet := subnetT.DeepCopy()
			subnet.Spec = spiderpoolv2beta1.SubnetSpec{
				IPVersion: ptr.To(int64(4)),
				Subnet:    "172.16.0.0/16",
				IPs:       []string{"172.16.41.1-172.16.41.200"},
			}
			subnet.Status = spiderpoolv2beta1.SubnetStatus{
				ControlledIPPools: ptr.To(`{"auto4-cloneset-demo-eth0-db543":{"ips":["172.16.41.1-172.16.41.2"],"application":"apps.kruise.io_v1alpha1_CloneSet_default_cloneset-demo"}}`),
				TotalIPCount:      ptr.To(int64(200)),
				AllocatedIPCount:  ptr.To(int64(2)),
			}

			patches := gomonkey.ApplyFuncReturn(applicationinformers.IsAppExist, true, types.UID("a-b-c"), nil)
			defer patches.Reset()

			err := fakeClient.Create(ctx, subnet)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnet)
			Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Add(subnet)
			Eventually(func(g Gomega) {
				var subnetR spiderpoolv2beta1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnet.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())
			}).Should(Succeed())
		})

		It("third-party controller exist with auto-created IPPool", func() {
			subnet := subnetT.DeepCopy()
			subnet.Spec = spiderpoolv2beta1.SubnetSpec{
				IPVersion: ptr.To(int64(4)),
				Subnet:    "172.16.0.0/16",
				IPs:       []string{"172.16.41.1-172.16.41.200"},
			}
			subnet.Status = spiderpoolv2beta1.SubnetStatus{
				ControlledIPPools: ptr.To(`{"auto4-cloneset-demo-eth0-db543":{"ips":["172.16.41.1-172.16.41.2"],"application":"apps.kruise.io_v1alpha1_CloneSet_default_cloneset-demo"}}`),
				TotalIPCount:      ptr.To(int64(200)),
				AllocatedIPCount:  ptr.To(int64(2)),
			}

			patches := gomonkey.ApplyFuncReturn(applicationinformers.IsAppExist, false, types.UID(""), nil)
			defer patches.Reset()

			err := fakeClient.Create(ctx, subnet)
			Expect(err).NotTo(HaveOccurred())

			err = subnetIndexer.Add(subnet)
			Expect(err).NotTo(HaveOccurred())

			fakeSubnetWatch.Add(subnet)
			Eventually(func(g Gomega) {
				var subnetR spiderpoolv2beta1.SpiderSubnet
				err = fakeClient.Get(ctx, types.NamespacedName{Name: subnet.Name}, &subnetR)
				g.Expect(err).NotTo(HaveOccurred())

				g.Expect(subnetR.Status.ControlledIPPools).To(BeNil())
			}).Should(Succeed())
		})

	})
})
