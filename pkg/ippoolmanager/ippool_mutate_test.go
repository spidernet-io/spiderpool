// Copyright 2025 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package ippoolmanager_test

import (
	"context"
	"fmt"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/ippoolmanager"
	spiderpoolv2beta1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v2beta1"
	"github.com/spidernet-io/spiderpool/pkg/logutils"
)

var _ = Describe("IPPoolWebhook iaas-provider label sync", Label("ippool_mutate_test"), func() {
	var ctx context.Context
	var count uint64
	var ipPoolName string
	var ipPoolT *spiderpoolv2beta1.SpiderIPPool

	BeforeEach(func() {
		ippoolmanager.WebhookLogger = logutils.Logger.Named("IPPool-Webhook")
		ipPoolWebhook.EnableIPv4 = true
		ipPoolWebhook.EnableIPv6 = true
		ipPoolWebhook.EnableSpiderSubnet = false

		ctx = context.TODO()

		atomic.AddUint64(&count, 1)
		ipPoolName = fmt.Sprintf("ippool-iaas-mutate-%v", count)
		ipPoolT = &spiderpoolv2beta1.SpiderIPPool{
			TypeMeta: metav1.TypeMeta{
				Kind:       constant.KindSpiderIPPool,
				APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersion),
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: ipPoolName,
			},
			Spec: spiderpoolv2beta1.IPPoolSpec{
				Subnet: "172.100.40.0/24",
			},
		}
	})

	AfterEach(func() {
		err := tracker.Delete(
			schema.GroupVersionResource{
				Group:    constant.SpiderpoolAPIGroup,
				Version:  constant.SpiderpoolAPIVersion,
				Resource: "spiderippools",
			},
			ipPoolT.Namespace,
			ipPoolT.Name,
		)
		Expect(client.IgnoreNotFound(err)).NotTo(HaveOccurred())
	})

	It("sets the label mirroring the iaas-provider annotation vendor value", func() {
		ipPoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: constant.IaasProviderHuaweiCloud,
		}

		err := ipPoolWebhook.Default(ctx, ipPoolT)
		Expect(err).NotTo(HaveOccurred())
		Expect(ipPoolT.Labels).To(HaveKeyWithValue(constant.LabelIPPoolIaasProvider, constant.IaasProviderHuaweiCloud))
	})

	It("corrects the label when the annotation value changes", func() {
		ipPoolT.Annotations = map[string]string{
			constant.AnnoIPPoolIaasProvider: constant.IaasProviderHuaweiCloud,
		}
		ipPoolT.Labels = map[string]string{
			constant.LabelIPPoolIaasProvider: "stale-vendor",
		}

		err := ipPoolWebhook.Default(ctx, ipPoolT)
		Expect(err).NotTo(HaveOccurred())
		Expect(ipPoolT.Labels).To(HaveKeyWithValue(constant.LabelIPPoolIaasProvider, constant.IaasProviderHuaweiCloud))
	})

	It("removes the label when the annotation is removed", func() {
		ipPoolT.Labels = map[string]string{
			constant.LabelIPPoolIaasProvider: constant.IaasProviderHuaweiCloud,
		}

		err := ipPoolWebhook.Default(ctx, ipPoolT)
		Expect(err).NotTo(HaveOccurred())
		Expect(ipPoolT.Labels).NotTo(HaveKey(constant.LabelIPPoolIaasProvider))
	})

	It("leaves a pool without the annotation unaffected", func() {
		err := ipPoolWebhook.Default(ctx, ipPoolT)
		Expect(err).NotTo(HaveOccurred())
		Expect(ipPoolT.Labels).NotTo(HaveKey(constant.LabelIPPoolIaasProvider))
	})
})
