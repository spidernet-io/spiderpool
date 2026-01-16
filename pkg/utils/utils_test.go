// Copyright 2024 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"encoding/json"
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Utils", Label("utils"), Serial, func() {
	clusterConfigurationJson := `{
	"apiVersion": "v1",
	"data": {
		"ClusterConfiguration": "networking:\n  dnsDomain: cluster.local\n  podSubnet: 192.168.165.0/24\n  serviceSubnet: 245.100.128.0/18"
	},
	"kind": "ConfigMap",
	"metadata": {
		"name": "kubeadm-config",
		"namespace": "kube-system"
	}
}`

	noClusterConfigurationJson := `{
	"apiVersion": "v1",
	"data": {
		"ClusterStatus": "apiEndpoints:\n  anolios79:\n    advertiseAddress: 192.168.165.128\n    bindPort: 6443\napiVersion: kubeadm.k8s.io/v1beta2\nkind: ClusterStatus\n"
	},
	"kind": "ConfigMap",
	"metadata": {
		"name": "kubeadm-config",
		"namespace": "kube-system"
	}
}`
	noCIDRJson := `{
	"apiVersion": "v1",
	"data": {
		"ClusterConfiguration": "clusterName: spider\ncontrolPlaneEndpoint: spider-control-plane:6443\ncontrollerManager:\n"
	},
	"kind": "ConfigMap",
	"metadata": {
		"name": "kubeadm-config",
		"namespace": "kube-system"
	}
}`
	DescribeTable("should extract CIDRs correctly",
		func(testName, cmStr string, expectedPodCIDR, expectedServiceCIDR []string, expectError bool) {
			var cm corev1.ConfigMap
			err := json.Unmarshal([]byte(cmStr), &cm)
			Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal configMap: %v\n", err)

			podCIDR, serviceCIDR, err := ExtractK8sCIDRFromKubeadmConfigMap(&cm)

			if expectError {
				Expect(err).To(HaveOccurred(), "Expected an error but got none")
			} else {
				Expect(err).NotTo(HaveOccurred(), "Did not expect an error but got one: %w", err)
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

	Context("GetDefaultCniName", func() {
		var cniFile string

		It("Test GetDefaultCniName", func() {
			cniFile = "10-calico.conflist"
			tempDir := "/tmp" + "/" + cniFile

			err := os.WriteFile(tempDir, []byte(`{
				"name": "calico",
				"cniVersion": "0.4.0",
				"plugins": [
					{
						"type": "calico",
						"etcd_endpoints": "http://127.0.0.1:2379",
						"log_level": "info",
						"ipam": {
							"type": "calico-ipam"
						},
						"policy": {
							"type": "k8s"
						}
					}
				]
			}`), 0o644)
			Expect(err).NotTo(HaveOccurred())
			cniName, err := GetDefaultCniName("/tmp")
			Expect(err).NotTo(HaveOccurred())

			Expect(cniName).To(Equal("calico"))
			// Expect(os.RemoveAll(tempCniDir)).NotTo(HaveOccurred())
		})
	})

	Describe("ExtractK8sCIDRFromKCMPod", func() {
		var pod *corev1.Pod

		BeforeEach(func() {
			pod = &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Command: []string{
								"/path/to/kube-controller-manager",
								"--cluster-cidr=192.168.0.0/16",
								"--service-cluster-ip-range=10.96.0.0/12",
							},
							Args: []string{
								"--cluster-cidr=192.168.0.0/16",
								"--service-cluster-ip-range=10.96.0.0/12",
							},
						},
					},
				},
			}
		})

		Context("when valid CIDR values are provided", func() {
			It("should extract pod CIDR and service CIDR correctly", func() {
				podCIDR, serviceCIDR := ExtractK8sCIDRFromKCMPod(pod)
				Expect(podCIDR).To(ConsistOf("192.168.0.0/16"))
				Expect(serviceCIDR).To(ConsistOf("10.96.0.0/12"))
			})
		})

		Context("when no CIDR values are provided", func() {
			BeforeEach(func() {
				pod.Spec.Containers[0].Command = []string{
					"/path/to/kube-controller-manager",
				}
				pod.Spec.Containers[0].Args = []string{}
			})

			It("should return empty slices for pod CIDR and service CIDR", func() {
				podCIDR, serviceCIDR := ExtractK8sCIDRFromKCMPod(pod)
				Expect(podCIDR).To(BeEmpty())
				Expect(serviceCIDR).To(BeEmpty())
			})
		})

		Context("when invalid CIDR values are provided", func() {
			BeforeEach(func() {
				pod.Spec.Containers[0].Command = []string{
					"/path/to/kube-controller-manager",
					"--cluster-cidr=invalidCIDR",
					"--service-cluster-ip-range=alsoInvalidCIDR",
				}
			})

			It("should return empty slices for pod CIDR and service CIDR", func() {
				podCIDR, serviceCIDR := ExtractK8sCIDRFromKCMPod(pod)
				Expect(podCIDR).To(BeEmpty())
				Expect(serviceCIDR).To(BeEmpty())
			})
		})

		Context("when multiple CIDR values are provided", func() {
			BeforeEach(func() {
				pod.Spec.Containers[0].Command = []string{
					"/path/to/kube-controller-manager",
					"--cluster-cidr=192.168.0.0/16,192.168.1.0/24",
					"--service-cluster-ip-range=10.96.0.0/12,10.97.0.0/16",
				}
			})

			It("should extract all valid pod CIDR and service CIDR", func() {
				podCIDR, serviceCIDR := ExtractK8sCIDRFromKCMPod(pod)
				Expect(podCIDR).To(ConsistOf("192.168.0.0/16", "192.168.1.0/24"))
				Expect(serviceCIDR).To(ConsistOf("10.96.0.0/12", "10.97.0.0/16"))
			})
		})
	})
})
