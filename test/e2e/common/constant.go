// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0

// Package common provides utility functions and constants for E2E tests.
package common

import (
	"os"
	"time"

	"github.com/spidernet-io/spiderpool/cmd/spiderpool/cmd"
)

// Defining K8s resource types
const (
	OwnerDeployment  string = "Deployment"
	OwnerStatefulSet string = "StatefulSet"
	OwnerDaemonSet   string = "DaemonSet"
	OwnerReplicaSet  string = "ReplicaSet"
	OwnerJob         string = "Job"
	OwnerPod         string = "Pod"
)

// Default timeouts to be used in context.WithTimeout
const (
	PodStartTimeout            = time.Minute * 5
	PodReStartTimeout          = time.Minute * 5
	IPReclaimTimeout           = time.Minute * 5
	ExecCommandTimeout         = time.Minute * 5
	EventOccurTimeout          = time.Second * 30
	ServiceAccountReadyTimeout = time.Minute
	NodeReadyTimeout           = time.Minute
	ResourceDeleteTimeout      = time.Minute * 5
	BatchCreateTimeout         = time.Minute * 5
	KdoctorCheckTime           = time.Minute * 10
	SpiderSyncMultusTime       = time.Minute * 2
	InformerSyncStatusTime     = time.Second * 30
	KDoctorRunTimeout          = time.Minute * 10
)

var ForcedWaitingTime = time.Second

// SpiderPool configurations
const (
	SpiderPoolConfigmapName      = "spiderpool-conf"
	SpiderPoolConfigmapNameSpace = "kube-system"
	SpiderPoolLeases             = "spiderpool-controller-leases"
	SpiderPoolLeasesNamespace    = "kube-system"
)

// Kubeadm configurations
const (
	KubeadmConfigmapName      = "kubeadm-config"
	KubeadmConfigmapNameSpace = "kube-system"
)

// Network configurations
var (
	// multus CNI
	MultusDefaultNetwork    = "v1.multus-cni.io/default-network"
	MultusNetworks          = "k8s.v1.cni.cncf.io/networks"
	PodMultusNetworksStatus = "k8s.v1.cni.cncf.io/network-status"

	CalicoCNIName               string = "k8s-pod-network"
	CiliumCNIName               string = "cilium"
	MacvlanUnderlayVlan0        string = "macvlan-vlan0"
	MacvlanVlan100              string = "macvlan-vlan100"
	MacvlanVlan200              string = "macvlan-vlan200"
	OvsVlan30                   string = "ovs-vlan30"
	OvsVlan40                   string = "ovs-vlan40"
	SpiderPoolIPv4SubnetVlan30  string = "vlan30-v4"
	SpiderPoolIPv6SubnetVlan30  string = "vlan30-v6"
	SpiderPoolIPv4SubnetVlan40  string = "vlan40-v4"
	SpiderPoolIPv6SubnetVlan40  string = "vlan40-v6"
	SpiderPoolIPv4SubnetDefault string = "default-v4-subnet"
	SpiderPoolIPv6SubnetDefault string = "default-v6-subnet"
	SpiderPoolIPv4PoolDefault   string = "default-v4-ippool"
	SpiderPoolIPv6PoolDefault   string = "default-v6-ippool"
	SpiderPoolIPv4SubnetVlan100 string = "vlan100-v4"
	SpiderPoolIPv6SubnetVlan100 string = "vlan100-v6"
	SpiderPoolIPv4SubnetVlan200 string = "vlan200-v4"
	SpiderPoolIPv6SubnetVlan200 string = "vlan200-v6"

	MultusNs                = "kube-system"
	KDoctorAgentNs          = "kube-system"
	KDoctorAgentDSName      = "kdoctor-agent"
	KDoctorAgentServiceIPV4 = "kdoctor-agent-ipv4"
	KDoctorAgentServiceIPV6 = "kdoctor-agent-ipv6"

	// gateway and check for ip conflicting machines
	VlanGatewayContainer = "vlan-gateway"

	// Network Name
	NIC1 string = "eth0"
	NIC2 string = "net1"
	NIC3 string = "eth0.100"
	NIC4 string = "eth0.200"
	NIC5 string = "eth1"
	NIC6 string = "net2"

	// Spidercoodinator podCIDRType
	PodCIDRTypeAuto    = "auto"
	PodCIDRTypeCluster = "cluster"
	PodCIDRTypeCalico  = "calico"
	PodCIDRTypeCilium  = "cilium"
	PodCIDRTypeNone    = "none"

	// Spidercoodinator default config
	SpidercoodinatorDefaultName = "default"
)

// Error
var (
	CNIFailedToSetUpNetwork = cmd.ErrPostIPAM.Error()
	GetIpamAllocationFailed = cmd.ErrPostIPAM.Error()
)

// The way to create an ippool
const (
	AutomaticallyCreated = "Automatic"
	ManuallyCreated      = "Manual"
)

// Webhook Port
const (
	WebhookPort                 = "5722"
	SpiderControllerMetricsPort = "5721"
	SpiderAgentMetricsPort      = "5711"
)

func init() {
	MultusNs = os.Getenv("RELEASE_NAMESPACE")
}
