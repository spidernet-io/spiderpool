// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
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
	ExecCommandTimeout         = time.Minute
	EventOccurTimeout          = time.Second * 30
	ServiceAccountReadyTimeout = time.Second * 20
	NodeReadyTimeout           = time.Minute
	ResourceDeleteTimeout      = time.Minute * 5
	BatchCreateTimeout         = time.Minute * 5
)

var ForcedWaitingTime = time.Second

// SpiderPool configurations
const (
	SpiderPoolConfigmapName      = "spiderpool-conf"
	SpiderPoolConfigmapNameSpace = "kube-system"
)

// Network configurations
var (
	// multus CNI
	MultusDefaultNetwork = "v1.multus-cni.io/default-network"
	MultusNetworks       = "k8s.v1.cni.cncf.io/networks"

	CalicoCNIName          string = "k8s-pod-network"
	CiliumCNIName          string = "cilium"
	MacvlanUnderlayVlan0   string = "macvlan-vlan0-underlay"
	MacvlanUnderlayVlan100 string = "macvlan-vlan100-underlay"
	MacvlanUnderlayVlan200 string = "macvlan-vlan200-underlay"
	MacvlanOverlayVlan100  string = "macvlan-vlan100-overlay"
	MacvlanOverlayVlan200  string = "macvlan-vlan200-overlay"

	SpiderPoolIPv4SubnetVlan100 string = "vlan100-v4"
	SpiderPoolIPv6SubnetVlan100 string = "vlan100-v6"
	SpiderPoolIPv4SubnetVlan200 string = "vlan200-v4"
	SpiderPoolIPv6SubnetVlan200 string = "vlan200-v6"

	SpiderPoolIPPoolAnnotationKey  = "ipam.spidernet.io/ippool"
	SpiderPoolIPPoolsAnnotationKey = "ipam.spidernet.io/ippools"
	SpiderPoolSubnetAnnotationKey  = "ipam.spidernet.io/subnet"
	SpiderPoolSubnetsAnnotationKey = "ipam.spidernet.io/subnets"

	MultusNs                = "kube-system"
	SpiderDoctorAgentNs     = "kube-system"
	SpiderDoctorAgentDSName = "spiderdoctor-agent"

	// Route
	V4Dst string = "0.0.0.0/0"
	V6Dst string = "::/0"

	// Network Name
	NIC1 string = "eth0"
	NIC2 string = "net1"
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
	WebhookPort = "5722"
)

func init() {
	MultusNs = os.Getenv("RELEASE_NAMESPACE")
}
