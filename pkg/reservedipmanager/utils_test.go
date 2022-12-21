// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"fmt"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolip "github.com/spidernet-io/spiderpool/pkg/ip"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/spiderpool.spidernet.io/v1"
	"github.com/spidernet-io/spiderpool/pkg/reservedipmanager"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("ReservedIPManager utils", Label("reservedip_manager_utils_test"), func() {
	Describe("Test AssembleReservedIPs", func() {
		var v4RIPT spiderpoolv1.SpiderReservedIP
		var v6RIPT spiderpoolv1.SpiderReservedIP
		var terminatingV4RIPT spiderpoolv1.SpiderReservedIP
		var rIPListT *spiderpoolv1.SpiderReservedIPList

		BeforeEach(func() {
			ipv4 := constant.IPv4
			ipv6 := constant.IPv6

			v4RIPT = spiderpoolv1.SpiderReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderReservedIPKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "ipv4-reservedip",
				},
				Spec: spiderpoolv1.ReservedIPSpec{
					IPVersion: &ipv4,
					IPs: []string{
						"172.18.40.1-172.18.40.2",
						"172.18.40.10",
					},
				},
			}

			v6RIPT = spiderpoolv1.SpiderReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderReservedIPKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "ipv6-reservedip",
				},
				Spec: spiderpoolv1.ReservedIPSpec{
					IPVersion: &ipv6,
					IPs: []string{
						"abcd:1234::1-abcd:1234::2",
						"abcd:1234::a",
					},
				},
			}

			now := metav1.Now()
			terminatingV4RIPT = spiderpoolv1.SpiderReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderReservedIPKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:                       "terminating-ipv4-reservedip",
					DeletionTimestamp:          &now,
					DeletionGracePeriodSeconds: pointer.Int64(30),
				},
				Spec: spiderpoolv1.ReservedIPSpec{
					IPVersion: &ipv4,
					IPs: []string{
						"172.18.40.40",
					},
				},
			}

			rIPListT = &spiderpoolv1.SpiderReservedIPList{
				TypeMeta: metav1.TypeMeta{
					Kind:       constant.SpiderReservedIPListKind,
					APIVersion: fmt.Sprintf("%s/%s", constant.SpiderpoolAPIGroup, constant.SpiderpoolAPIVersionV1),
				},
				ListMeta: metav1.ListMeta{},
				Items:    []spiderpoolv1.SpiderReservedIP{},
			}
		})

		It("inputs invalid IP version", func() {
			ips, err := reservedipmanager.AssembleReservedIPs(constant.InvalidIPVersion, rIPListT)
			Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPVersion))
			Expect(ips).To(BeEmpty())
		})

		It("inputs nil ReservedIP list", func() {
			ips, err := reservedipmanager.AssembleReservedIPs(constant.IPv4, nil)
			Expect(err).To(MatchError(constant.ErrMissingRequiredParam))
			Expect(ips).To(BeEmpty())
		})

		It("inputs invalid IP ranges", func() {
			v4RIPT.Spec.IPs = append(v4RIPT.Spec.IPs, constant.InvalidIPRange)
			rIPListT.Items = append(rIPListT.Items, v4RIPT)

			ips, err := reservedipmanager.AssembleReservedIPs(constant.IPv4, rIPListT)
			Expect(err).To(MatchError(spiderpoolip.ErrInvalidIPRangeFormat))
			Expect(ips).To(BeEmpty())
		})

		It("does not assemble terminating IPv4 reserved-IP addresses", func() {
			rIPListT.Items = append(rIPListT.Items, v4RIPT, terminatingV4RIPT)

			ips, err := reservedipmanager.AssembleReservedIPs(constant.IPv4, rIPListT)
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal(
				[]net.IP{
					net.IPv4(172, 18, 40, 1),
					net.IPv4(172, 18, 40, 2),
					net.IPv4(172, 18, 40, 10),
				},
			))
		})

		It("assembles IPv4 reserved-IP addresses", func() {
			rIPListT.Items = append(rIPListT.Items, v4RIPT, v6RIPT)

			ips, err := reservedipmanager.AssembleReservedIPs(constant.IPv4, rIPListT)
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal(
				[]net.IP{
					net.IPv4(172, 18, 40, 1),
					net.IPv4(172, 18, 40, 2),
					net.IPv4(172, 18, 40, 10),
				},
			))
		})

		It("assembles IPv6 reserved-IP addresses", func() {
			rIPListT.Items = append(rIPListT.Items, v4RIPT, v6RIPT)

			ips, err := reservedipmanager.AssembleReservedIPs(constant.IPv6, rIPListT)
			Expect(err).NotTo(HaveOccurred())
			Expect(ips).To(Equal(
				[]net.IP{
					net.ParseIP("abcd:1234::1"),
					net.ParseIP("abcd:1234::2"),
					net.ParseIP("abcd:1234::a"),
				},
			))
		})
	})
})
