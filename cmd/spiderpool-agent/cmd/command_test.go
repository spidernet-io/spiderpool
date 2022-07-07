// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	_ "github.com/spidernet-io/spiderpool/api/v1/agent/models"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type IPMock string

const (
	v4_mock IPMock = "192.168.2.1"
	v6_mock IPMock = "2345:425:2CA1::567:5673:23b5"
)

var _ = Describe("spiderpool agent", Label("unitest", "ipam_agent_command_test"), func() {

	Context("Testing reservedIPManager", func() {

		It("should return an error if client is not created in reservedIPManager", func() {
			reservedIPRanges, err := rIPManager.GetReservedIPRanges(ctx, "IPv4")
			Expect(apierrors.IsNotFound(err)).Should(BeFalse())
			Expect(reservedIPRanges).To(BeNil())
		})

		It("should return the reserved ip ranges with correct Ips", func() {
			ipv4 := spiderpoolv1.IPv4
			ipv6 := spiderpoolv1.IPv6
			Ipv4_client := &spiderpoolv1.ReservedIP{
				ObjectMeta: metav1.ObjectMeta{
					Name: "ipv4client",
				},

				Spec: spiderpoolv1.ReservedIPSpec{
					IPVersion: &ipv4,
					IPs:       []string{string(v4_mock)},
				},
			}
			Ipv6_client := &spiderpoolv1.ReservedIP{
				TypeMeta: metav1.TypeMeta{
					Kind: "ReservedIP",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name: "ipv6client",
				},
				Spec: spiderpoolv1.ReservedIPSpec{
					IPVersion: &ipv6,
					IPs:       []string{string(v6_mock)},
				},
			}

			err = k8sClient.Create(ctx, Ipv4_client)
			Expect(err).Should(BeNil())
			err = k8sClient.Create(ctx, Ipv6_client)
			Expect(err).Should(BeNil())

			reservedIPRanges, err := rIPManager.GetReservedIPRanges(ctx, "IPv4")
			Expect(err).Should(BeNil())

			for _, ips := range reservedIPRanges {
				Expect(govalidator.IsIPv4(ips)).Should(BeTrue())
			}

		})

	})

})
