// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/agiledragon/gomonkey/v2"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/spidernet-io/spiderpool/pkg/applicationcontroller/applicationinformers"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta2"
	spiderpoolfake "github.com/spidernet-io/spiderpool/pkg/k8s/client/clientset/versioned/fake"
	"github.com/spidernet-io/spiderpool/pkg/k8s/client/informers/externalversions"
	"github.com/spidernet-io/spiderpool/pkg/metric"
)

var _ = Describe("IPPool-informer", Label("unittest"), Ordered, func() {
	BeforeAll(func() {
		_, err := metric.InitMetric(context.TODO(), constant.SpiderpoolController, false, false)
		Expect(err).NotTo(HaveOccurred())
		err = metric.InitSpiderpoolControllerMetrics(context.TODO())
		Expect(err).NotTo(HaveOccurred())
	})

	var pool *spiderpoolv2beta1.SpiderIPPool
	BeforeEach(func() {
		pool = &spiderpoolv2beta1.SpiderIPPool{
			TypeMeta: metav1.TypeMeta{
				Kind:       constant.KindSpiderIPPool,
				APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:   "test-ippool",
				Labels: map[string]string{},
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{
				IPVersion: ptr.To(int64(4)),
				Subnet:    "10.1.0.0/16",
				IPs:       []string{"10.1.0.1-10.1.0.10"},
			},
			Status: spiderpoolv2beta1.IPPoolStatus{
				AllocatedIPs:     nil,
				TotalIPCount:     ptr.To(int64(10)),
				AllocatedIPCount: ptr.To(int64(0)),
			},
		}
	})

	Describe("run ippool controller", func() {
		var control *poolController
		BeforeEach(func() {
			scheme = runtime.NewScheme()
			err := spiderpoolv2beta1.AddToScheme(scheme)
			Expect(err).NotTo(HaveOccurred())
			err = appsv1.AddToScheme(scheme)
			Expect(err).NotTo(HaveOccurred())

			control = newController()
			fakeClientSet := spiderpoolfake.NewSimpleClientset()
			factory := externalversions.NewSharedInformerFactory(fakeClientSet, 0)
			err = control.addEventHandlers(factory.Spiderpool().V2beta1().SpiderIPPools())
			Expect(err).NotTo(HaveOccurred())
			control.ipPoolStore = factory.Spiderpool().V2beta1().SpiderIPPools().Informer().GetStore()
		})

		Context("enqueue an IPPool", func() {
			It("enqueue an normal IPPool that informer synced", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				pool.Status = spiderpoolv2beta1.IPPoolStatus{}

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.ipPoolStore.Add(pool)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueIPPool(pool)
			})

			It("enqueue an normal IPPool that informer didn't synced", func() {
				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				control.enqueueIPPool(pool)
			})

			It("enqueue an auto-created IPPool that informer synced", func() {
				tmpPool := pool.DeepCopy()
				tmpPool.Name = "auto4-deploy-abc"
				labels := map[string]string{
					constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(appsv1.SchemeGroupVersion.String()),
					constant.LabelIPPoolOwnerApplicationKind:      constant.KindDeployment,
					constant.LabelIPPoolOwnerApplicationNamespace: "test-ns",
					constant.LabelIPPoolOwnerApplicationName:      "test-name",
					constant.LabelIPPoolReclaimIPPool:             constant.True,
				}
				tmpPool.SetLabels(labels)

				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.ipPoolStore.Add(tmpPool)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueIPPool(tmpPool)
			})

			It("enqueue an terminating auto-created IPPool that informer synced", func() {
				tmpPool := pool.DeepCopy()
				tmpPool.Name = "auto4-deploy-abc"
				now := metav1.Now()
				tmpPool.DeletionTimestamp = now.DeepCopy()
				labels := map[string]string{
					constant.LabelIPPoolOwnerApplicationGV:        applicationinformers.ApplicationLabelGV(appsv1.SchemeGroupVersion.String()),
					constant.LabelIPPoolOwnerApplicationKind:      constant.KindDeployment,
					constant.LabelIPPoolOwnerApplicationNamespace: "test-ns",
					constant.LabelIPPoolOwnerApplicationName:      "test-name",
					constant.LabelIPPoolReclaimIPPool:             constant.True,
				}
				tmpPool.SetLabels(labels)
				controllerutil.AddFinalizer(tmpPool, constant.SpiderFinalizer)

				ctx, cancel := context.WithCancel(context.TODO())
				defer cancel()

				go func() {
					defer GinkgoRecover()

					patches := gomonkey.ApplyFuncReturn(cache.WaitForCacheSync, true)
					defer patches.Reset()

					err := control.Run(ctx.Done())
					if nil != err {
						cancel()
						Fail(err.Error())
					}
				}()

				time.Sleep(time.Second)
				err := control.ipPoolStore.Add(tmpPool)
				Expect(err).NotTo(HaveOccurred())
				control.enqueueIPPool(tmpPool)
			})
		})

	})

})

var scheme *runtime.Scheme
var poolControllerConfig IPPoolControllerConfig

type poolController struct {
	*IPPoolController
	ipPoolStore cache.Store
}

func newController() *poolController {
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(scheme)

	poolControllerConfig = IPPoolControllerConfig{
		IPPoolControllerWorkers:       3,
		EnableSpiderSubnet:            true,
		MaxWorkqueueLength:            5000,
		WorkQueueMaxRetries:           10,
		LeaderRetryElectGap:           0,
		WorkQueueRequeueDelayDuration: -1 * time.Second,
		ResyncPeriod:                  10 * time.Second,
	}

	pController := NewIPPoolController(poolControllerConfig, fakeClient, fakeDynamicClient)

	return &poolController{
		IPPoolController: pController,
	}
}
