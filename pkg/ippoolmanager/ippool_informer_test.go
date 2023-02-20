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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	spiderpoolfake "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned/fake"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var _ = Describe("IPPoolInformer", Label("ippool_informer_test"), func() {
	Describe("New IPPoolController", func() {
		It("sets default IPPoolController", func() {
			controller := ippoolmanager.NewIPPoolController(ippoolmanager.IPPoolControllerConfig{}, fakeClient, rIPManager)
			Expect(controller).NotTo(BeNil())
		})

		It("inputs nil client", func() {
			controller := ippoolmanager.NewIPPoolController(ippoolmanager.IPPoolControllerConfig{}, nil, rIPManager)
			Expect(controller).NotTo(BeNil())
		})

		It("inputs nil reserved-IP manager", func() {
			manager := ippoolmanager.NewIPPoolController(ippoolmanager.IPPoolControllerConfig{}, fakeClient, nil)
			Expect(manager).NotTo(BeNil())
		})
	})

	Describe("Test IPPoolInformer's method", func() {
		var count uint64
		var ipPoolName string
		var ipPoolT *spiderpoolv1.SpiderIPPool

		var ippoolController *ippoolmanager.IPPoolController
		var fakeIPPoolWatch *watch.FakeWatcher
		// var ipPoolIndexer cache.Indexer

		BeforeEach(func() {
			ippoolmanager.InformerLogger = logutils.Logger.Named("Ippool-Informer")
			atomic.AddUint64(&count, 1)
			ipPoolName = fmt.Sprintf("ippool-%v", count)
			ipPoolT = &spiderpoolv1.SpiderIPPool{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderIPPoolKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   ipPoolName,
					Labels: map[string]string{},
				},
				Spec: spiderpoolv1.IPPoolSpec{},
			}

			ippoolController = &ippoolmanager.IPPoolController{}

			patches := gomonkey.ApplyFuncReturn(cache.WaitForNamedCacheSync, true)
			DeferCleanup(patches.Reset)

			mockLeaderElector.EXPECT().
				IsElected().
				Return(true).
				AnyTimes()

			ctx, cancel := context.WithCancel(context.Background())
			DeferCleanup(cancel)

			fakeIPPoolWatch = watch.NewFake()
			fakeClientset := spiderpoolfake.NewSimpleClientset()
			fakeClientset.PrependWatchReactor("spiderippools", testing.DefaultWatchReactor(fakeIPPoolWatch, nil))

			err := ippoolController.SetupInformer(ctx, fakeClientset, mockLeaderElector)
			Expect(err).NotTo(HaveOccurred())
			Eventually(func(g Gomega) {
				g.Expect(ippoolController.IPPoolIndexer).NotTo(BeNil())
				// ipPoolIndexer = ippoolController.IPPoolIndexer
			}).Should(Succeed())

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
		})

		It("inputs nil setupInformer's controllerLeader", func() {
			ctx := context.TODO()
			fakeClientset := spiderpoolfake.NewSimpleClientset()
			fakeClientset.PrependWatchReactor("spiderippools", testing.DefaultWatchReactor(fakeIPPoolWatch, nil))
			err := ippoolController.SetupInformer(ctx, fakeClientset, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
		})
	})
})
