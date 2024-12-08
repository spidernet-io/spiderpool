// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Coordinator Manager", Label("coordinatorinformer", "informer_test"), Serial, func() {
	DescribeTable("should extract CIDRs correctly",
		func(testName, cmStr string, expectedPodCIDR, expectedServiceCIDR []string, expectError bool) {
			var cm corev1.ConfigMap
			err := json.Unmarshal([]byte(cmStr), &cm)
			Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal configMap: %v\n", err)

			podCIDR, serviceCIDR, err := ExtractK8sCIDRFromKubeadmConfigMap(&cm)

			if expectError {
				Expect(err).To(HaveOccurred(), "Expected an error but got none")
			} else {
				Expect(err).NotTo(HaveOccurred(), "Did not expect an error but got one: %v", err)
			}

			Expect(podCIDR).To(Equal(expectedPodCIDR), "Pod CIDR does not match")
			Expect(serviceCIDR).To(Equal(expectedServiceCIDR), "Service CIDR does not match")
		},
		Entry("ClusterConfiguration",
			"ClusterConfiguration",
			clusterConfigurationJson,
			[]string{"192.168.165.0/24"},
			[]string{"245.100.128.0/18"},
			false,
		),
		Entry("No ClusterConfiguration",
			"No ClusterConfiguration",
			noClusterConfigurationJson,
			nil,
			nil,
			true,
		),
		Entry("No CIDR",
			"No CIDR",
			noCIDRJson,
			nil,
			nil,
			false,
		),
	)

})
