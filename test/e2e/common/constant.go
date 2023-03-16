// Copyright 2022 Authors of spidernet-io
// SPDX-License-Identifier: Apache-2.0
package common

import (
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
	PodStartTimeout            = time.Minute * 2
	PodReStartTimeout          = time.Minute * 2
	IPReclaimTimeout           = time.Minute * 2
	ExecCommandTimeout         = time.Minute
	EventOccurTimeout          = time.Second * 30
	ServiceAccountReadyTimeout = time.Second * 20
	NodeReadyTimeout           = time.Minute
	ResourceDeleteTimeout      = time.Minute * 2
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
	MultusNetworks string = "k8s.v1.cni.cncf.io/networks"
	MacvlanCNIName string = "kube-system/macvlan-cni2"

	// Route
	V4Dst string = "0.0.0.0/0"
	V6Dst string = "::/0"

	// Network Name
	NIC1 string = "eth0"
	NIC2 string = "net2"
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
