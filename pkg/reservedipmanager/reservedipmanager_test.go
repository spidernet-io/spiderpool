// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package reservedipmanager_test

import (
	"github.com/asaskevich/govalidator"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spidernet-io/spiderpool/pkg/constant"
	spiderpoolv1 "github.com/spidernet-io/spiderpool/pkg/k8s/apis/v1"
	"github.com/spidernet-io/spiderpool/pkg/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	v4_mock string = "127.1.0.5"
	v6_mock string = "0000:0000:0000:0000:0000:0000:0000:0001"
)

var _ = Describe("Reservedipmanager", Label("unittest", "Reservedipmanager"), func() {
	It("It should return an error if client is not created in reservedIPManager", func() {
		reservedIPRanges, err := rIPManager.GetReservedIPByName(ctx, "ipv4client")
		Expect(apierrors.IsNotFound(err)).Should(BeTrue())
		Expect(reservedIPRanges).To(BeNil())
	})

	It("It should return the reserved ip ranges with correct Ips", func() {
		v4IP := new(types.IPVersion)
		*v4IP = constant.IPv4
		v6IP := new(types.IPVersion)
		*v6IP = constant.IPv6
		Ipv4_client := &spiderpoolv1.ReservedIP{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ipv4client",
			},

			Spec: spiderpoolv1.ReservedIPSpec{
				IPVersion: v4IP,
				IPs:       []string{v4_mock},
			},
		}
		Ipv6_client := &spiderpoolv1.ReservedIP{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ipv6client",
			},
			Spec: spiderpoolv1.ReservedIPSpec{
				IPVersion: v6IP,
				IPs:       []string{v6_mock},
			},
		}

		err = k8sClient.Create(ctx, Ipv4_client)
		Expect(err).Should(BeNil())
		err = k8sClient.Create(ctx, Ipv6_client)
		Expect(err).Should(BeNil())

		reservedIPRanges, err := rIPManager.GetReservedIPByName(ctx, "ipv4client")
		Expect(reservedIPRanges).NotTo(BeNil())
		Expect(err).ShouldNot(HaveOccurred())

		rIPList, err := rIPManager.ListReservedIPs(ctx)
		Expect(err).ShouldNot(HaveOccurred())
		reservedIPs, err := rIPManager.GetReservedIPsByIPVersion(ctx, constant.IPv4, rIPList)
		Expect(err).ShouldNot(HaveOccurred())

		for _, ips := range reservedIPs {
			Expect(govalidator.IsIPv4(ips.String())).Should(BeTrue())
		}

	})
})
