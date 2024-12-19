// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

package coordinatormanager

import (
	"encoding/json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("Coordinator Manager", Label("unittest", "informer_test"), Serial, func() {

	var (
		clusterConfigurationInOneLineCm *corev1.ConfigMap
		clusterConfigurationJsonCm      *corev1.ConfigMap
		noClusterConfigurationJsonCm    *corev1.ConfigMap
		noCIDRJsonCm                    *corev1.ConfigMap
	)

	BeforeEach(func() {
		clusterConfigurationInOneLineJson := `
{"apiVersion":"v1","data":{"ClusterConfiguration":"apiServer:\n  certSANs:\n  - 127.0.0.1\n  - apiserver.cluster.local\n  - 10.103.97.2\n  - 192.168.165.128\n  extraArgs:\n    audit-log-format: json\n    audit-log-maxage: \"7\"\n    audit-log-maxbackup: \"10\"\n    audit-log-maxsize: \"100\"\n    audit-log-path: /var/log/kubernetes/audit.log\n    audit-policy-file: /etc/kubernetes/audit-policy.yml\n    authorization-mode: Node,RBAC\n    enable-aggregator-routing: \"true\"\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/kubernetes\n    mountPath: /etc/kubernetes\n    name: audit\n    pathType: DirectoryOrCreate\n  - hostPath: /var/log/kubernetes\n    mountPath: /var/log/kubernetes\n    name: audit-log\n    pathType: DirectoryOrCreate\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\n  timeoutForControlPlane: 4m0s\napiVersion: kubeadm.k8s.io/v1beta2\ncertificatesDir: /etc/kubernetes/pki\nclusterName: kubernetes\ncontrolPlaneEndpoint: apiserver.cluster.local:6443\ncontrollerManager:\n  extraArgs:\n    bind-address: 0.0.0.0\n    cluster-signing-duration: 876000h\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\ndns:\n  type: CoreDNS\netcd:\n  local:\n    dataDir: /var/lib/etcd\n    extraArgs:\n      listen-metrics-urls: http://0.0.0.0:2381\nimageRepository: k8s.gcr.io\nkind: ClusterConfiguration\nkubernetesVersion: v1.21.14\nnetworking:\n  dnsDomain: cluster.local\n  podSubnet: 192.168.165.0/24\n  serviceSubnet: 245.100.128.0/18\nscheduler:\n  extraArgs:\n    bind-address: 0.0.0.0\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\n","ClusterStatus":"apiEndpoints:\n  anolios79:\n    advertiseAddress: 192.168.165.128\n    bindPort: 6443\napiVersion: kubeadm.k8s.io/v1beta2\nkind: ClusterStatus\n"},"kind":"ConfigMap","metadata":{"name":"kubeadm-config","namespace":"kube-system"}}`
		clusterConfigurationJson := `{
    "apiVersion": "v1",
    "data": {
        "ClusterConfiguration": "apiServer:\n  certSANs:\n  - 127.0.0.1\n  - apiserver.cluster.local\n  - 10.103.97.2\n  - 192.168.165.128\n  extraArgs:\n    audit-log-format: json\n    audit-log-maxage: \"7\"\n    audit-log-maxbackup: \"10\"\n    audit-log-maxsize: \"100\"\n    audit-log-path: /var/log/kubernetes/audit.log\n    audit-policy-file: /etc/kubernetes/audit-policy.yml\n    authorization-mode: Node,RBAC\n    enable-aggregator-routing: \"true\"\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/kubernetes\n    mountPath: /etc/kubernetes\n    name: audit\n    pathType: DirectoryOrCreate\n  - hostPath: /var/log/kubernetes\n    mountPath: /var/log/kubernetes\n    name: audit-log\n    pathType: DirectoryOrCreate\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\n  timeoutForControlPlane: 4m0s\napiVersion: kubeadm.k8s.io/v1beta2\ncertificatesDir: /etc/kubernetes/pki\nclusterName: kubernetes\ncontrolPlaneEndpoint: apiserver.cluster.local:6443\ncontrollerManager:\n  extraArgs:\n    bind-address: 0.0.0.0\n    cluster-signing-duration: 876000h\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\ndns:\n  type: CoreDNS\netcd:\n  local:\n    dataDir: /var/lib/etcd\n    extraArgs:\n      listen-metrics-urls: http://0.0.0.0:2381\nimageRepository: k8s.gcr.io\nkind: ClusterConfiguration\nkubernetesVersion: v1.21.14\nnetworking:\n  dnsDomain: cluster.local\n  podSubnet: 192.168.165.0/24\n  serviceSubnet: 245.100.128.0/18\nscheduler:\n  extraArgs:\n    bind-address: 0.0.0.0\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\n",
        "ClusterStatus": "apiEndpoints:\n  anolios79:\n    advertiseAddress: 192.168.165.128\n    bindPort: 6443\napiVersion: kubeadm.k8s.io/v1beta2\nkind: ClusterStatus\n"
    },
    "kind": "ConfigMap",
    "metadata": {
        "name": "kubeadm-config",
        "namespace": "kube-system",
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
        "namespace": "kube-system",
    }
}`
		noCIDRJson := `{
    "apiVersion": "v1",
    "data": {
        "ClusterConfiguration": "apiServer:\n  certSANs:\n  - 127.0.0.1\n  - apiserver.cluster.local\n  - 10.103.97.2\n  - 192.168.165.128\n  extraArgs:\n    audit-log-format: json\n    audit-log-maxage: \"7\"\n    audit-log-maxbackup: \"10\"\n    audit-log-maxsize: \"100\"\n    audit-log-path: /var/log/kubernetes/audit.log\n    audit-policy-file: /etc/kubernetes/audit-policy.yml\n    authorization-mode: Node,RBAC\n    enable-aggregator-routing: \"true\"\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/kubernetes\n    mountPath: /etc/kubernetes\n    name: audit\n    pathType: DirectoryOrCreate\n  - hostPath: /var/log/kubernetes\n    mountPath: /var/log/kubernetes\n    name: audit-log\n    pathType: DirectoryOrCreate\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\n  timeoutForControlPlane: 4m0s\napiVersion: kubeadm.k8s.io/v1beta2\ncertificatesDir: /etc/kubernetes/pki\nclusterName: kubernetes\ncontrolPlaneEndpoint: apiserver.cluster.local:6443\ncontrollerManager:\n  extraArgs:\n    bind-address: 0.0.0.0\n    cluster-signing-duration: 876000h\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\ndns:\n  type: CoreDNS\netcd:\n  local:\n    dataDir: /var/lib/etcd\n    extraArgs:\n      listen-metrics-urls: http://0.0.0.0:2381\nimageRepository: k8s.gcr.io\nkind: ClusterConfiguration\nkubernetesVersion: v1.21.14\nnetworking:\n  dnsDomain: cluster.local\nscheduler:\n  extraArgs:\n    bind-address: 0.0.0.0\n    feature-gates: EphemeralContainers=true,TTLAfterFinished=true\n  extraVolumes:\n  - hostPath: /etc/localtime\n    mountPath: /etc/localtime\n    name: localtime\n    pathType: File\n    readOnly: true\n",
        "ClusterStatus": "apiEndpoints:\n  anolios79:\n    advertiseAddress: 192.168.165.128\n    bindPort: 6443\napiVersion: kubeadm.k8s.io/v1beta2\nkind: ClusterStatus\n"
    },
    "kind": "ConfigMap",
    "metadata": {
        "name": "kubeadm-config",
        "namespace": "kube-system",
    }
}`

		// unmarshal json to corev1.ConfigMap
		clusterConfigurationInOneLineCm = &corev1.ConfigMap{}
		err := json.Unmarshal([]byte(clusterConfigurationInOneLineJson), clusterConfigurationInOneLineCm)
		Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal clusterConfigurationInOneLineJson")

		clusterConfigurationJsonCm = &corev1.ConfigMap{}
		err = json.Unmarshal([]byte(clusterConfigurationJson), clusterConfigurationJsonCm)
		Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal clusterConfigurationJson")

		noClusterConfigurationJsonCm = &corev1.ConfigMap{}
		err = json.Unmarshal([]byte(noClusterConfigurationJson), noClusterConfigurationJsonCm)
		Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal noClusterConfigurationJson")

		noCIDRJsonCm = &corev1.ConfigMap{}
		err = json.Unmarshal([]byte(noCIDRJson), noCIDRJsonCm)
		Expect(err).NotTo(HaveOccurred(), "Failed to unmarshal noCIDRJson")
	})

	DescribeTable("should extract CIDRs correctly",
		func(testName string, cm *corev1.ConfigMap, expectedPodCIDR, expectedServiceCIDR []string, expectError bool) {
			It(testName, func() {
				podCIDR, serviceCIDR, err := ExtractK8sCIDRFromKubeadmConfigMap(cm)

				if expectError {
					Expect(err).To(HaveOccurred(), "Expected an error but got none")
				} else {
					Expect(err).NotTo(HaveOccurred(), "Did not expect an error but got one: %v", err)
				}

				Expect(podCIDR).To(Equal(expectedPodCIDR), "Pod CIDR does not match")
				Expect(serviceCIDR).To(Equal(expectedServiceCIDR), "Service CIDR does not match")
			})
		},
		Entry("ClusterConfiguration In One line",
			"ClusterConfiguration In One line",
			clusterConfigurationInOneLineCm,
			[]string{"192.168.165.0/24"},
			[]string{"245.100.128.0/18"},
			false,
		),
		Entry("ClusterConfiguration",
			"ClusterConfiguration",
			clusterConfigurationJsonCm,
			[]string{"192.168.165.0/24"},
			[]string{"245.100.128.0/18"},
			false,
		),
		Entry("No ClusterConfiguration",
			"No ClusterConfiguration",
			noClusterConfigurationJsonCm,
			nil,
			nil,
			true,
		),
		Entry("No CIDR",
			"No CIDR",
			noCIDRJsonCm,
			nil,
			nil,
			false,
		),
	)
})
