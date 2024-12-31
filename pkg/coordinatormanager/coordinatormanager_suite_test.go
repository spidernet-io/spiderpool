// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var (
	clusterConfigurationJson   string
	noClusterConfigurationJson string
	noCIDRJson                 string
)

func TestCoordinatorManager(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "CoordinatorManager Suite")
}

var _ = BeforeSuite(func() {
	clusterConfigurationJson = `
	{
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
	noClusterConfigurationJson = `
	{
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
	noCIDRJson = `
	{
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
})
