// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package namespacemanager_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	"github.com/spidernet-io/spiderpool/pkg/namespacemanager"
)

var _ = Describe("NamespaceManager utils", Label("namespace_manager_utils_test"), func() {
	Describe("Test GetNSDefaultPools", func() {
		var v4Pool1, v4Pool2 string
		var v6Pool1, v6Pool2 string
		var nsT *corev1.Namespace

		BeforeEach(func() {
			v4Pool1, v4Pool2 = "ns-default-ipv4-ippool1", "ns-default-ipv4-ippool2"
			v6Pool1, v6Pool2 = "ns-default-ipv6-ippool1", "ns-default-ipv6-ippool2"

			nsT = &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Namespace",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace",
				},
				Spec: corev1.NamespaceSpec{},
			}
		})

		It("inputs nil Namespace", func() {
			nsDefaultV4Pools, nsDefaultV6Pools, err := namespacemanager.GetNSDefaultPools(nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(nsDefaultV4Pools).To(BeEmpty())
			Expect(nsDefaultV6Pools).To(BeEmpty())
		})

		It("inputs invalid annotation ipam.spidernet.io/default-ipv4-ippool", func() {
			nsT.SetAnnotations(map[string]string{
				constant.AnnoNSDefautlV4Pool: "invalid value",
			})
			nsDefaultV4Pools, nsDefaultV6Pools, err := namespacemanager.GetNSDefaultPools(nsT)
			Expect(err).To(HaveOccurred())
			Expect(nsDefaultV4Pools).To(BeEmpty())
			Expect(nsDefaultV6Pools).To(BeEmpty())
		})

		It("inputs invalid annotation ipam.spidernet.io/default-ipv6-ippool", func() {
			nsT.SetAnnotations(map[string]string{
				constant.AnnoNSDefautlV6Pool: "invalid value",
			})
			nsDefaultV4Pools, nsDefaultV6Pools, err := namespacemanager.GetNSDefaultPools(nsT)
			Expect(err).To(HaveOccurred())
			Expect(nsDefaultV4Pools).To(BeEmpty())
			Expect(nsDefaultV6Pools).To(BeEmpty())
		})

		It("sets the IPv4 namespace default pools", func() {
			nsT.SetAnnotations(map[string]string{
				constant.AnnoNSDefautlV4Pool: fmt.Sprintf(`["%s", "%s"]`, v4Pool1, v4Pool2),
			})
			nsDefaultV4Pools, nsDefaultV6Pools, err := namespacemanager.GetNSDefaultPools(nsT)
			Expect(err).NotTo(HaveOccurred())
			Expect(nsDefaultV4Pools).To(Equal([]string{v4Pool1, v4Pool2}))
			Expect(nsDefaultV6Pools).To(BeEmpty())
		})

		It("sets the IPv6 namespace default pools", func() {
			nsT.SetAnnotations(map[string]string{
				constant.AnnoNSDefautlV6Pool: fmt.Sprintf(`["%s", "%s"]`, v6Pool1, v6Pool2),
			})
			nsDefaultV4Pools, nsDefaultV6Pools, err := namespacemanager.GetNSDefaultPools(nsT)
			Expect(err).NotTo(HaveOccurred())
			Expect(nsDefaultV4Pools).To(BeEmpty())
			Expect(nsDefaultV6Pools).To(Equal([]string{v6Pool1, v6Pool2}))
		})

		It("sets the dual-stack namespace default pools", func() {
			nsT.SetAnnotations(map[string]string{
				constant.AnnoNSDefautlV4Pool: fmt.Sprintf(`["%s", "%s"]`, v4Pool1, v4Pool2),
				constant.AnnoNSDefautlV6Pool: fmt.Sprintf(`["%s"]`, v6Pool1),
			})
			nsDefaultV4Pools, nsDefaultV6Pools, err := namespacemanager.GetNSDefaultPools(nsT)
			Expect(err).NotTo(HaveOccurred())
			Expect(nsDefaultV4Pools).To(Equal([]string{v4Pool1, v4Pool2}))
			Expect(nsDefaultV6Pools).To(Equal([]string{v6Pool1}))
		})
	})
})
